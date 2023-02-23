package tendermint

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/AstraProtocol/astra-indexing/external/cache"
	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"

	"github.com/AstraProtocol/astra-indexing/appinterface/tendermint"

	"github.com/AstraProtocol/astra-indexing/usecase/model/genesis"

	usecase_model "github.com/AstraProtocol/astra-indexing/usecase/model"
	"github.com/hashicorp/go-retryablehttp"
)

var _ tendermint.Client = &HTTPClient{}

var (
	redirectsErrorRe = regexp.MustCompile(`stopped after \d+ redirects\z`)
	// specifically so we resort to matching on the error string.
	schemeErrorRe = regexp.MustCompile(`unsupported protocol scheme`)

	// A regular expression to match the error returned by net/http when the
	// TLS certificate is not trusted. This error isn't typed
	// specifically so we resort to matching on the error string.
	notTrustedErrorRe = regexp.MustCompile(`certificate is not trusted`)
)

type HTTPClient struct {
	httpClient           *retryablehttp.Client
	tendermintRPCUrl     string
	strictGenesisParsing bool
	httpCache            *cache.AstraCache
}

// NewHTTPClient returns a new HTTPClient for tendermint request
func NewHTTPClient(tendermintRPCUrl string, strictGenesisParsing bool) *HTTPClient {
	httpClient := retryablehttp.NewClient()
	httpClient.Logger = nil
	httpClient.CheckRetry = defaultRetryPolicy

	return &HTTPClient{
		httpClient,
		strings.TrimSuffix(tendermintRPCUrl, "/"),
		strictGenesisParsing,
		cache.NewCache(),
	}
}

// defaultRetryPolicy provides a default callback for Client.CheckRetry, which
// will retry on connection errors and server errors.
func defaultRetryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	// do not retry on context.Canceled or context.DeadlineExceeded
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	// don't propagate other errors
	shouldRetry, _ := baseRetryPolicy(resp, err)
	return shouldRetry, nil
}

func baseRetryPolicy(resp *http.Response, err error) (bool, error) {
	if err != nil {
		if v, ok := err.(*url.Error); ok {
			// Don't retry if the error was due to too many redirects.
			if redirectsErrorRe.MatchString(v.Error()) {
				return false, v
			}

			// Don't retry if the error was due to an invalid protocol scheme.
			if schemeErrorRe.MatchString(v.Error()) {
				return false, v
			}

			// Don't retry if the error was due to TLS cert verification failure.
			if notTrustedErrorRe.MatchString(v.Error()) {
				return false, v
			}
			if _, ok := v.Err.(x509.UnknownAuthorityError); ok {
				return false, v
			}
		}

		// The error is likely recoverable so retry.
		return true, nil
	}

	// 429 Too Many Requests is recoverable. Sometimes the server puts
	// a Retry-After response header to indicate when the server is
	// available to start processing request from client.
	if resp.StatusCode == http.StatusTooManyRequests {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return true, nil
	}
	// Check the response code. We retry on 500-range responses to allow
	// the server time to recover, as 500's are typically not permanent
	// errors and may relate to outages on the server side. This will catch
	// invalid response codes as well, like 0 and 999.
	if resp.StatusCode == 0 || (resp.StatusCode >= 500 && resp.StatusCode != http.StatusNotImplemented) {
		return true, fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}

	return false, nil
}

func (client *HTTPClient) Genesis() (*genesis.Genesis, error) {
	var err error

	rawRespBody, err := client.request("genesis")
	if err != nil {
		return nil, err
	}
	defer rawRespBody.Close()

	genesisResp, err := ParseGenesisResp(rawRespBody, client.strictGenesisParsing)
	if err != nil {
		return nil, err
	}

	return genesisResp, nil
}

// Block gets the block response with target height
func (client *HTTPClient) Block(height int64) (*usecase_model.Block, *usecase_model.RawBlock, error) {
	var err error

	rawRespBody, err := client.request("block", "height="+strconv.FormatInt(height, 10))

	if err != nil {
		return nil, nil, err
	}
	defer rawRespBody.Close()

	block, rawBlock, err := ParseBlockResp(rawRespBody)
	if err != nil {
		return nil, nil, err
	}

	return block, rawBlock, nil
}

func (client *HTTPClient) BlockResults(height int64) (*usecase_model.BlockResults, error) {
	cacheBlockResults := usecase_model.BlockResults{}
	var err error
	err = client.httpCache.Get(strconv.FormatInt(height, 10), &cacheBlockResults)
	if err == nil {
		return &cacheBlockResults, nil
	}
	rawRespBody, err := client.request("block_results", "height="+strconv.FormatInt(height, 10))
	if err != nil {
		return nil, err
	}
	defer rawRespBody.Close()

	blockResults, err := ParseBlockResultsResp(rawRespBody)
	if err != nil {
		return nil, err
	}
	_ = client.httpCache.Set(strconv.FormatInt(height, 10), blockResults, time.Minute)
	return blockResults, nil
}

// LatestBlockHeight gets the chain's latest block and return the height
func (client *HTTPClient) LatestBlockHeight() (int64, error) {
	var err error
	rawRespBody, err := client.request("block")
	if err != nil {
		return int64(0), fmt.Errorf("error getting /block: %v", err)
	}
	defer rawRespBody.Close()

	block, _, err := ParseBlockResp(rawRespBody)
	if err != nil {
		return int64(0), fmt.Errorf("error parsing /block response: %v", err)
	}

	return block.Height, nil
}

// request construct tendermint url and issues an HTTP request
// returns the success http Body
func (client *HTTPClient) request(method string, queryString ...string) (io.ReadCloser, error) {
	var err error
	startTime := time.Now()
	queryUrl := client.tendermintRPCUrl + "/" + method
	if len(queryString) > 0 {
		queryUrl += "?" + queryString[0]
	}

	req, err := retryablehttp.NewRequestWithContext(context.Background(), http.MethodGet, queryUrl, nil)
	if err != nil {
		prometheus.RecordApiExecTime(method, strconv.Itoa(-1), "rpc", time.Since(startTime).Milliseconds())
		return nil, fmt.Errorf("error creating HTTP request with context: %v", err)
	}
	rawResp, err := client.httpClient.Do(req)
	if err != nil {
		prometheus.RecordApiExecTime(method, strconv.Itoa(-1), "rpc", time.Since(startTime).Milliseconds())
		return nil, fmt.Errorf("error requesting Tendermint %s endpoint: %v", queryUrl, err)
	}
	prometheus.RecordApiExecTime(method, strconv.Itoa(rawResp.StatusCode), "rpc", time.Since(startTime).Milliseconds())
	if rawResp.StatusCode != 200 {
		rawResp.Body.Close()
		return nil, fmt.Errorf("error requesting Tendermint %s endpoint: %s", method, rawResp.Status)
	}

	return rawResp.Body, nil
}

func (client *HTTPClient) Status() (*map[string]interface{}, error) {
	rawRespBody, err := client.request("status")
	if err != nil {
		return nil, err
	}
	defer rawRespBody.Close()

	body, _ := ioutil.ReadAll(rawRespBody)
	jsonMap := make(map[string]interface{})
	errRead := json.Unmarshal([]byte(body), &jsonMap)
	if errRead != nil {
		return nil, fmt.Errorf("error requesting Status : %v", errRead)
	}

	return &jsonMap, nil
}

type GenesisResp struct {
	Jsonrpc string            `json:"jsonrpc"`
	ID      int64             `json:"id"`
	Result  GenesisRespResult `json:"result"`
}

type GenesisRespResult struct {
	Genesis genesis.Genesis `json:"genesis"`
}

type RawBlockResp struct {
	Jsonrpc string                 `json:"jsonrpc"`
	ID      int                    `json:"id"`
	Result  usecase_model.RawBlock `json:"result"`
}

type RawBlockResultsResp struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  RawBlockResults `json:"result"`
}
