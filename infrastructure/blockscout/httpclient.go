package blockscout

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/AstraProtocol/astra-indexing/external/cache"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/hashicorp/go-retryablehttp"
	jsoniter "github.com/json-iterator/go"
)

const GET_DETAIL_EVM_TX = "/api/v1?module=transaction&action=getTxCosmosInfo&txhash="
const GET_SEARCH_RESULTS = "/token-autocomplete?q="
const TX_NOT_FOUND = "transaction not found"

type HTTPClient struct {
	logger     applogger.Logger
	httpClient *retryablehttp.Client
	url        string
	httpCache  *cache.AstraCache
}

var (
	redirectsErrorRe  = regexp.MustCompile(`stopped after \d+ redirects\z`)
	schemeErrorRe     = regexp.MustCompile(`unsupported protocol scheme`)
	notTrustedErrorRe = regexp.MustCompile(`certificate is not trusted`)
)

func defaultRetryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	shouldRetry, _ := baseRetryPolicy(resp, err)
	return shouldRetry, nil
}

func baseRetryPolicy(resp *http.Response, err error) (bool, error) {
	if err != nil {
		if v, ok := err.(*url.Error); ok {
			if redirectsErrorRe.MatchString(v.Error()) {
				return false, v
			}
			if schemeErrorRe.MatchString(v.Error()) {
				return false, v
			}
			if notTrustedErrorRe.MatchString(v.Error()) {
				return false, v
			}
			if _, ok := v.Err.(x509.UnknownAuthorityError); ok {
				return false, v
			}
		}
		return true, nil
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return true, nil
	}
	if resp.StatusCode == 0 || (resp.StatusCode >= 500 && resp.StatusCode != http.StatusNotImplemented) {
		return true, fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}
	return false, nil
}

func (client *HTTPClient) getUrl(action string, param string) string {
	return fmt.Sprintf("%s%s%s", client.url, action, param)
}

func (client *HTTPClient) request(endpoint string, queryParams ...string) (io.ReadCloser, error) {
	var err error
	queryUrl := endpoint

	if len(queryParams[0]) > 0 {
		for _, v := range queryParams {
			queryUrl += "?" + v
		}
	}

	req, err := retryablehttp.NewRequestWithContext(context.Background(), http.MethodGet, queryUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request with context: %v", err)
	}
	rawResp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error requesting blockscout %s endpoint: %v", queryUrl, err)
	}

	if rawResp.StatusCode != 200 {
		rawResp.Body.Close()
		return nil, fmt.Errorf("error requesting blockscout %s endpoint: %s", queryUrl, rawResp.Status)
	}

	return rawResp.Body, nil
}

func NewHTTPClient(logger applogger.Logger, url string) *HTTPClient {
	httpClient := retryablehttp.NewClient()
	httpClient.Logger = nil
	httpClient.CheckRetry = defaultRetryPolicy

	return &HTTPClient{
		logger.WithFields(applogger.LogFields{
			"module": "BlockscoutHttpClient",
		}),
		httpClient,
		strings.TrimSuffix(url, "/"),
		cache.NewCache("blockscout"),
	}
}

func (client *HTTPClient) GetDetailEvmTx(txHash string) (*TransactionEvm, error) {
	rawRespBody, err := client.request(
		client.getUrl(GET_DETAIL_EVM_TX, txHash), "",
	)
	if err != nil {
		return nil, err
	}
	defer rawRespBody.Close()

	var txResp TxResp
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&txResp); err != nil {
		return nil, err
	}

	if txResp.Status == "0" {
		return nil, fmt.Errorf(TX_NOT_FOUND)
	}

	return &txResp.Result, nil
}

func (client *HTTPClient) GetSearchResultsAsync(keyword string, results chan []SearchResult) {
	// Make sure we close these channels when we're done with them\\
	defer func() {
		close(results)
	}()

	rawRespBody, err := client.request(
		client.getUrl(GET_SEARCH_RESULTS, keyword), "",
	)
	if err != nil {
		client.logger.Errorf("error getting search results from blockscout: %v", err)
		results <- []SearchResult{}
		return
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var seachResults []SearchResult
	if err := json.Unmarshal(respBody.Bytes(), &seachResults); err != nil {
		client.logger.Errorf("error parsing search results from blockscout: %v", err)
	}
	results <- seachResults
}
