package jsonrpc

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/AstraProtocol/astra-indexing/external/cache"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	utils "github.com/AstraProtocol/astra-indexing/infrastructure"
	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"
	"github.com/hashicorp/go-retryablehttp"
	jsoniter "github.com/json-iterator/go"
)

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
		return false, nil
	}
	if resp.StatusCode == 0 || (resp.StatusCode >= 500 && resp.StatusCode != http.StatusNotImplemented) {
		return true, fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}
	return false, nil
}

func (client *HTTPClient) getUrl(action string, param string) string {
	return fmt.Sprintf("%s%s%s", client.url, action, param)
}

func (client *HTTPClient) requestPost(endpoint string, rawBody interface{}) (io.ReadCloser, error) {
	startTime := time.Now()
	var err error

	req, err := retryablehttp.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, rawBody)
	if err != nil {
		prometheus.RecordApiExecTime(endpoint, strconv.Itoa(408), "http", time.Since(startTime).Milliseconds())
		return nil, fmt.Errorf("error creating HTTP request with context: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	rawResp, err := client.httpClient.Do(req)
	if err != nil {
		prometheus.RecordApiExecTime(endpoint, strconv.Itoa(400), "http", time.Since(startTime).Milliseconds())
		return nil, fmt.Errorf("error requesting blockscout %s endpoint: %v", endpoint, err)
	}

	prometheus.RecordApiExecTime(endpoint, strconv.Itoa(rawResp.StatusCode), "http", time.Since(startTime).Milliseconds())

	return rawResp.Body, nil
}

func NewHTTPClient(logger applogger.Logger, url string) *HTTPClient {
	httpClient := retryablehttp.NewClient()
	httpClient.Logger = nil
	httpClient.CheckRetry = defaultRetryPolicy
	httpClient.RetryMax = 0

	return &HTTPClient{
		logger.WithFields(applogger.LogFields{
			"module": "APIRPCHttpClient",
		}),
		httpClient,
		strings.TrimSuffix(url, "/"),
		cache.NewCache(),
	}
}

func (client *HTTPClient) EthCall(bodyParams interface{}) (interface{}, error) {
	cacheKey := fmt.Sprintf("JsonrpcEthCall_%s", fmt.Sprint(bodyParams))

	var commonRespTmp CommonResp
	err := client.httpCache.Get(cacheKey, &commonRespTmp)
	if err == nil {
		return commonRespTmp, nil
	}

	postBody, err := json.Marshal(bodyParams)
	if err != nil {
		return nil, err
	}

	rawRespBody, err := client.requestPost(client.getUrl("", ""), postBody)
	if err != nil {
		return nil, err
	}
	defer rawRespBody.Close()

	var commonResp CommonResp
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&commonResp); err != nil {
		return nil, err
	}

	client.httpCache.Set(cacheKey, commonResp, utils.TIME_CACHE_FAST)

	return commonResp, nil
}
