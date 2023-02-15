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
	"strconv"
	"strings"
	"time"

	"github.com/AstraProtocol/astra-indexing/external/cache"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"
	"github.com/hashicorp/go-retryablehttp"
	jsoniter "github.com/json-iterator/go"
)

const GET_DETAIL_EVM_TX_BY_COSMOS_TX_HASH = "/api/v1?module=transaction&action=getTxCosmosInfo&txhash="
const GET_DETAIL_EVM_TX_BY_EVM_TX_HASH = "/api/v1?module=transaction&action=gettxinfo&txhash="
const GET_LIST_INTERNAL_TXS_BY_EVM_TX_HASH = "/api/v1?module=account&action=txlistinternal&txhash="
const GET_DETAIL_ADDRESS_BY_ADDRESS_HASH = "/api/v1?module=account&action=getaddress&address="
const GET_ADDRESS_COUNTERS = "/api/v1?module=account&action=getaddresscounters&address="
const GET_TOP_ADDRESSES_BALANCE = "/api/v1?module=account&action=getTopAddressesBalance"
const GET_LIST_TOKENS = "/api/v1?module=token&action=getListTokens"
const GET_COMMON_STATS = "/api/v1/common-stats"
const GET_SEARCH_RESULTS = "/token-autocomplete?q="
const ETH_BLOCK_NUMBER = "/api/v1?module=block&action=eth_block_number"
const TX_NOT_FOUND = "transaction not found"
const ADDRESS_NOT_FOUND = "address not found"
const DEFAULT_PAGE = 1
const DEFAULT_OFFSET = 10

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

func (client *HTTPClient) request(endpoint string, queryParams []string, mappingParams map[string]string) (io.ReadCloser, error) {
	startTime := time.Now()
	var err error
	queryUrl := endpoint

	if len(queryParams) > 0 {
		for _, v := range queryParams {
			queryUrl += "&" + v
			queryUrl += "=" + mappingParams[v]
		}
	}

	req, err := retryablehttp.NewRequestWithContext(context.Background(), http.MethodGet, queryUrl, nil)
	if err != nil {
		prometheus.RecordApiExecTime(queryUrl, strconv.Itoa(-1), "http", time.Since(startTime).Milliseconds())
		return nil, fmt.Errorf("error creating HTTP request with context: %v", err)
	}
	rawResp, err := client.httpClient.Do(req)
	if err != nil {
		prometheus.RecordApiExecTime(queryUrl, strconv.Itoa(-1), "http", time.Since(startTime).Milliseconds())
		return nil, fmt.Errorf("error requesting blockscout %s endpoint: %v", queryUrl, err)
	}

	prometheus.RecordApiExecTime(queryUrl, strconv.Itoa(rawResp.StatusCode), "http", time.Since(startTime).Milliseconds())

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
	httpClient.RetryMax = 0

	return &HTTPClient{
		logger.WithFields(applogger.LogFields{
			"module": "BlockscoutHttpClient",
		}),
		httpClient,
		strings.TrimSuffix(url, "/"),
		cache.NewCache("blockscout"),
	}
}

func (client *HTTPClient) GetDetailEvmTxByCosmosTxHash(txHash string) (*TransactionEvm, error) {
	rawRespBody, err := client.request(
		client.getUrl(GET_DETAIL_EVM_TX_BY_COSMOS_TX_HASH, txHash), nil, nil,
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

func (client *HTTPClient) GetDetailEvmTxByEvmTxHash(evmTxHash string) (*TransactionEvm, error) {
	rawRespBody, err := client.request(
		client.getUrl(GET_DETAIL_EVM_TX_BY_EVM_TX_HASH, evmTxHash), nil, nil,
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

func (client *HTTPClient) GetDetailEvmTxByCosmosTxHashAsync(evmTxHash string, transactionEvmRespChan chan TxResp) {
	// Make sure we close these channels when we're done with them
	defer func() {
		close(transactionEvmRespChan)
	}()

	rawRespBody, err := client.request(
		client.getUrl(GET_DETAIL_EVM_TX_BY_COSMOS_TX_HASH, evmTxHash), nil, nil,
	)
	if err != nil {
		client.logger.Errorf("error getting transaction evm by cosmos tx hash from blockscout: %v", err)
		transactionEvmRespChan <- TxResp{}
		return
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var transactionEvmResp TxResp
	if err := json.Unmarshal(respBody.Bytes(), &transactionEvmResp); err != nil {
		client.logger.Errorf("error parsing transaction evm by cosmos tx hash from blockscout: %v", err)
	}

	transactionEvmRespChan <- transactionEvmResp
}

func (client *HTTPClient) GetDetailEvmTxByEvmTxHashAsync(evmTxHash string, transactionEvmRespChan chan TxResp) {
	// Make sure we close these channels when we're done with them
	defer func() {
		close(transactionEvmRespChan)
	}()

	rawRespBody, err := client.request(
		client.getUrl(GET_DETAIL_EVM_TX_BY_EVM_TX_HASH, evmTxHash), nil, nil,
	)
	if err != nil {
		client.logger.Errorf("error getting transaction evm by evm tx hash from blockscout: %v", err)
		transactionEvmRespChan <- TxResp{}
		return
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var transactionEvmResp TxResp
	if err := json.Unmarshal(respBody.Bytes(), &transactionEvmResp); err != nil {
		client.logger.Errorf("error parsing transaction evm by evm tx hash from blockscout: %v", err)
	}

	transactionEvmRespChan <- transactionEvmResp
}

func (client *HTTPClient) GetCommonStatsAsync(commonStatsChan chan CommonStats) {
	// Make sure we close these channels when we're done with them
	defer func() {
		close(commonStatsChan)
	}()

	rawRespBody, err := client.request(
		client.getUrl(GET_COMMON_STATS, ""), nil, nil,
	)
	if err != nil {
		client.logger.Errorf("error getting common stats from blockscout: %v", err)
		commonStatsChan <- CommonStats{}
		return
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var commonStats CommonStats
	if err := json.Unmarshal(respBody.Bytes(), &commonStats); err != nil {
		client.logger.Errorf("error parsing common stats from blockscout: %v", err)
	}
	commonStatsChan <- commonStats
}

func (client *HTTPClient) GetAddressCountersAsync(addressHash string, addressCountersChan chan AddressCounterResp) {
	// Make sure we close these channels when we're done with them
	defer func() {
		close(addressCountersChan)
	}()

	rawRespBody, err := client.request(
		client.getUrl(GET_ADDRESS_COUNTERS, addressHash), nil, nil,
	)
	if err != nil {
		client.logger.Errorf("error getting address counters from blockscout: %v", err)
		addressCountersChan <- AddressCounterResp{}
		return
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var addressCounterResp AddressCounterResp
	if err := json.Unmarshal(respBody.Bytes(), &addressCounterResp); err != nil {
		client.logger.Errorf("error parsing address counters from blockscout: %v", err)
	}
	addressCountersChan <- addressCounterResp
}

func (client *HTTPClient) GetDetailAddressByAddressHashAsync(addressHash string, addressChan chan AddressResp) {
	// Make sure we close these channels when we're done with them
	defer func() {
		close(addressChan)
	}()

	rawRespBody, err := client.request(
		client.getUrl(GET_DETAIL_ADDRESS_BY_ADDRESS_HASH, addressHash), nil, nil,
	)
	if err != nil {
		client.logger.Errorf("error getting address detail from blockscout: %v", err)
		addressChan <- AddressResp{}
		return
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var addressResp AddressResp
	if err := json.Unmarshal(respBody.Bytes(), &addressResp); err != nil {
		client.logger.Errorf("error parsing address detail from blockscout: %v", err)
	}
	addressChan <- addressResp
}

func (client *HTTPClient) GetSearchResults(keyword string) []SearchResult {
	rawRespBody, err := client.request(
		client.getUrl(GET_SEARCH_RESULTS, keyword), nil, nil,
	)
	if err != nil {
		client.logger.Errorf("error getting search results from blockscout: %v", err)
		return []SearchResult{}
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var seachResults []SearchResult
	if err := json.Unmarshal(respBody.Bytes(), &seachResults); err != nil {
		client.logger.Errorf("error parsing search results from blockscout: %v", err)
		return []SearchResult{}
	}

	return seachResults
}

func (client *HTTPClient) GetSearchResultsAsync(keyword string, results chan []SearchResult) {
	// Make sure we close these channels when we're done with them
	defer func() {
		close(results)
	}()

	rawRespBody, err := client.request(
		client.getUrl(GET_SEARCH_RESULTS, keyword), nil, nil,
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

func (client *HTTPClient) GetTopAddressesBalance(queryParams []string, mappingParams map[string]string) (*TopAddressesBalanceResp, error) {
	rawRespBody, err := client.request(
		client.getUrl(GET_TOP_ADDRESSES_BALANCE, ""), queryParams, mappingParams,
	)
	if err != nil {
		client.logger.Errorf("error getting top addresses balance from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var topAddressesBalanceResp TopAddressesBalanceResp
	if err := json.Unmarshal(respBody.Bytes(), &topAddressesBalanceResp); err != nil {
		client.logger.Errorf("error parsing top addresses balance from blockscout: %v", err)
	}

	return &topAddressesBalanceResp, nil
}

func (client *HTTPClient) EthBlockNumber() (*EthBlockNumber, error) {
	rawRespBody, err := client.request(
		client.getUrl(ETH_BLOCK_NUMBER, ""), nil, nil,
	)
	if err != nil {
		client.logger.Errorf("error getting eth block number from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var ethBlockNumber EthBlockNumber
	if err := json.Unmarshal(respBody.Bytes(), &ethBlockNumber); err != nil {
		client.logger.Errorf("error parsing eth block number from blockscout: %v", err)
	}

	return &ethBlockNumber, nil
}

func (client *HTTPClient) GetListTokens(queryParams []string, mappingParams map[string]string) (*ListTokenResp, error) {
	rawRespBody, err := client.request(
		client.getUrl(GET_LIST_TOKENS, ""), queryParams, mappingParams,
	)
	if err != nil {
		client.logger.Errorf("error getting list tokens from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var listTokenResp ListTokenResp
	if err := json.Unmarshal(respBody.Bytes(), &listTokenResp); err != nil {
		client.logger.Errorf("error parsing list token from blockscout: %v", err)
	}

	return &listTokenResp, nil
}

func (client *HTTPClient) GetListInternalTxs(evmTxHash string) ([]InternalTransaction, error) {
	rawRespBody, err := client.request(
		client.getUrl(GET_LIST_INTERNAL_TXS_BY_EVM_TX_HASH, evmTxHash), nil, nil,
	)
	if err != nil {
		return nil, err
	}
	defer rawRespBody.Close()

	var internalTxsResp InternalTxsResp
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&internalTxsResp); err != nil {
		return nil, err
	}

	if internalTxsResp.Status == "0" {
		return nil, fmt.Errorf(TX_NOT_FOUND)
	}

	return internalTxsResp.Result, nil
}
