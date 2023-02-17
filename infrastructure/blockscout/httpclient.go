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
const GET_ABI_BY_ADDRESS_HASH = "/api/v1?module=contract&action=getabi&address="
const GET_ABI_BY_TX_HASH = "/api/v1?module=transaction&action=getabibytxhash&txhash="
const GET_DETAIL_ADDRESS_BY_ADDRESS_HASH = "/api/v1?module=account&action=getaddress&address="
const GET_ADDRESS_COUNTERS = "/api/v1?module=account&action=getaddresscounters&address="
const GET_TOP_ADDRESSES_BALANCE = "/api/v1?module=account&action=getTopAddressesBalance"
const GET_LIST_TOKENS = "/api/v1?module=token&action=getListTokens"
const GET_COMMON_STATS = "/api/v1/common-stats"
const GET_SEARCH_RESULTS = "/token-autocomplete?q="
const ETH_BLOCK_NUMBER = "/api/v1?module=block&action=eth_block_number"
const MARKET_HISTORY_CHART = "/api/v1/market-history-chart"
const GAS_PRICE_ORACLE = "/api/v1/gas-price-oracle"
const ADDRESS_COIN_BALANCE_HISTORY_CHART = "/address/{addresshash}/coin-balances/by-day?type=JSON"
const GET_RAW_TRACE_BY_TX_HASH = "/api/v1?module=transaction&action=getrawtracebytxhash&txhash="
const GET_LIST_TOKEN_OF_AN_ADDRESS = "/api/v1?module=account&action=tokenlist&address="
const GET_ADDRESS_COIN_BALANCE_HISTORY = "/api/v1?module=account&action=getcoinbalancehistory&address="
const GET_LIST_INTERNAL_TXS_BY_ADDRESS_HASH = "/api/v1?module=account&action=txlistinternal&address="
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
	cacheKey := fmt.Sprintf("BlockscoutGetDetailEvmTxByCosmosTxHash_%s", txHash)
	var transactionEvmTmp TransactionEvm

	err := client.httpCache.Get(cacheKey, &transactionEvmTmp)
	if err == nil {
		return &transactionEvmTmp, nil
	}

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

	client.httpCache.Set(cacheKey, &txResp.Result, 10*60*1000*time.Millisecond)

	return &txResp.Result, nil
}

func (client *HTTPClient) GetDetailEvmTxByEvmTxHash(evmTxHash string) (*TransactionEvm, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetDetailEvmTxByEvmTxHash_%s", evmTxHash)
	var transactionEvmTmp TransactionEvm

	err := client.httpCache.Get(cacheKey, &transactionEvmTmp)
	if err == nil {
		return &transactionEvmTmp, nil
	}

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

	client.httpCache.Set(cacheKey, &txResp.Result, 10*60*1000*time.Millisecond)

	return &txResp.Result, nil
}

func (client *HTTPClient) GetDetailEvmTxByCosmosTxHashAsync(txHash string, transactionEvmRespChan chan TxResp) {
	cacheKey := fmt.Sprintf("BlockscoutGetDetailEvmTxByCosmosTxHashAsync_%s", txHash)
	var txRespTmp TxResp

	err := client.httpCache.Get(cacheKey, &txRespTmp)
	if err == nil {
		transactionEvmRespChan <- txRespTmp
		return
	}

	// Make sure we close these channels when we're done with them
	defer func() {
		close(transactionEvmRespChan)
	}()

	rawRespBody, err := client.request(
		client.getUrl(GET_DETAIL_EVM_TX_BY_COSMOS_TX_HASH, txHash), nil, nil,
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

	client.httpCache.Set(cacheKey, transactionEvmResp, 10*60*1000*time.Millisecond)

	transactionEvmRespChan <- transactionEvmResp
}

func (client *HTTPClient) GetDetailEvmTxByEvmTxHashAsync(evmTxHash string, transactionEvmRespChan chan TxResp) {
	cacheKey := fmt.Sprintf("BlockscoutGetDetailEvmTxByEvmTxHashAsync_%s", evmTxHash)
	var txRespTmp TxResp

	err := client.httpCache.Get(cacheKey, &txRespTmp)
	if err == nil {
		transactionEvmRespChan <- txRespTmp
		return
	}

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

	client.httpCache.Set(cacheKey, transactionEvmResp, 10*60*1000*time.Millisecond)

	transactionEvmRespChan <- transactionEvmResp
}

func (client *HTTPClient) GetCommonStatsAsync(commonStatsChan chan CommonStats) {
	cacheKey := "BlockscoutGetCommonStatsAsync"
	var commonStatsTmp CommonStats

	err := client.httpCache.Get(cacheKey, &commonStatsTmp)
	if err == nil {
		commonStatsChan <- commonStatsTmp
		return
	}

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

	client.httpCache.Set(cacheKey, commonStats, 10*60*1000*time.Millisecond)

	commonStatsChan <- commonStats
}

func (client *HTTPClient) GetAddressCountersAsync(addressHash string, addressCountersChan chan AddressCounterResp) {
	cacheKey := fmt.Sprintf("BlockscoutGetAddressCountersAsync_%s", addressHash)
	var addressCounterRespTmp AddressCounterResp

	err := client.httpCache.Get(cacheKey, &addressCounterRespTmp)
	if err == nil {
		addressCountersChan <- addressCounterRespTmp
		return
	}

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

	client.httpCache.Set(cacheKey, addressCounterResp, 30*1000*time.Millisecond)

	addressCountersChan <- addressCounterResp
}

func (client *HTTPClient) GetDetailAddressByAddressHashAsync(addressHash string, addressChan chan AddressResp) {
	cacheKey := fmt.Sprintf("BlockscoutGetDetailAddressByAddressHashAsync_%s", addressHash)
	var addressRespTmp AddressResp

	err := client.httpCache.Get(cacheKey, &addressRespTmp)
	if err == nil {
		addressChan <- addressRespTmp
		return
	}

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

	client.httpCache.Set(cacheKey, addressResp, 10*60*1000*time.Millisecond)

	addressChan <- addressResp
}

func (client *HTTPClient) GetSearchResults(keyword string) []SearchResult {
	cacheKey := fmt.Sprintf("BlockscoutGetSearchResults_%s", keyword)
	var searchResultsTmp []SearchResult

	err := client.httpCache.Get(cacheKey, &searchResultsTmp)
	if err == nil {
		return searchResultsTmp
	}

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

	client.httpCache.Set(cacheKey, seachResults, 10*60*1000*time.Millisecond)

	return seachResults
}

func (client *HTTPClient) GetSearchResultsAsync(keyword string, results chan []SearchResult) {
	cacheKey := fmt.Sprintf("BlockscoutGetSearchResultsAsync_%s", keyword)
	var searchResultsTmp []SearchResult

	err := client.httpCache.Get(cacheKey, &searchResultsTmp)
	if err == nil {
		results <- searchResultsTmp
		return
	}

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

	client.httpCache.Set(cacheKey, seachResults, 10*60*1000*time.Millisecond)

	results <- seachResults
}

func (client *HTTPClient) GetTopAddressesBalance(queryParams []string, mappingParams map[string]string) (*TopAddressesBalanceResp, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetTopAddressesBalance_%s_%s", mappingParams["page"], mappingParams["offset"])
	var topAddressesBalanceRespTmp TopAddressesBalanceResp

	err := client.httpCache.Get(cacheKey, &topAddressesBalanceRespTmp)
	if err == nil {
		return &topAddressesBalanceRespTmp, nil
	}

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

	client.httpCache.Set(cacheKey, &topAddressesBalanceResp, 10*60*1000*time.Millisecond)

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
	cacheKey := fmt.Sprintf("BlockscouGetListTokens_%s_%s", mappingParams["page"], mappingParams["offset"])
	var listTokenRespTmp ListTokenResp

	err := client.httpCache.Get(cacheKey, &listTokenRespTmp)
	if err == nil {
		return &listTokenRespTmp, nil
	}

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

	client.httpCache.Set(cacheKey, &listTokenResp, 10*60*1000*time.Millisecond)

	return &listTokenResp, nil
}

func (client *HTTPClient) GetListInternalTxs(evmTxHash string) ([]InternalTransaction, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetListInternalTxs_%s", evmTxHash)
	var internalTxsTmp []InternalTransaction

	err := client.httpCache.Get(cacheKey, &internalTxsTmp)
	if err == nil {
		return internalTxsTmp, nil
	}

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

	client.httpCache.Set(cacheKey, internalTxsResp.Result, 60*1000*time.Millisecond)

	return internalTxsResp.Result, nil
}

func (client *HTTPClient) GetAbiByAddressHash(addressHash string) (string, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetAbiByAddressHash_%s", addressHash)
	var abiTmp string

	err := client.httpCache.Get(cacheKey, &abiTmp)
	if err == nil {
		return abiTmp, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(GET_ABI_BY_ADDRESS_HASH, addressHash), nil, nil,
	)
	if err != nil {
		return "", err
	}
	defer rawRespBody.Close()

	var abiResp AccountAbiResp
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&abiResp); err != nil {
		return "", err
	}

	if abiResp.Status == "0" {
		return "", fmt.Errorf(ADDRESS_NOT_FOUND)
	}

	client.httpCache.Set(cacheKey, abiResp.Result, 10*60*1000*time.Millisecond)

	return abiResp.Result, nil
}

func (client *HTTPClient) GetAbiByTransactionHash(txHash string) (*AbiResult, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetAbiByTransactionHash_%s", txHash)
	var abiResultTmp AbiResult

	err := client.httpCache.Get(cacheKey, &abiResultTmp)
	if err == nil {
		return &abiResultTmp, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(GET_ABI_BY_TX_HASH, txHash), nil, nil,
	)

	if err != nil {
		return nil, err
	}
	defer rawRespBody.Close()

	var abiResp TxAbiResp
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&abiResp); err != nil {
		return nil, err
	}

	if abiResp.Status == "0" {
		return nil, fmt.Errorf(TX_NOT_FOUND)
	}

	client.httpCache.Set(cacheKey, &abiResp.Result, 10*60*1000*time.Millisecond)

	return &abiResp.Result, nil
}

func (client *HTTPClient) MarketHistoryChart() (*MarketHistory, error) {
	cacheKey := "BlockscoutMarketHistoryChart"
	var marketHistoryTmp MarketHistory

	err := client.httpCache.Get(cacheKey, &marketHistoryTmp)
	if err == nil {
		return &marketHistoryTmp, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(MARKET_HISTORY_CHART, ""), nil, nil,
	)
	if err != nil {
		client.logger.Errorf("error getting market history chart from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var marketHistory MarketHistory
	if err := json.Unmarshal(respBody.Bytes(), &marketHistory); err != nil {
		client.logger.Errorf("error parsing market history chart from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, marketHistory, 60*60*1000*time.Millisecond)

	return &marketHistory, nil
}

func (client *HTTPClient) GasPriceOracle() (*GasPriceOracle, error) {
	cacheKey := "BlockscoutGasPriceOracle"
	var gasPriceOracleTmp GasPriceOracle

	err := client.httpCache.Get(cacheKey, &gasPriceOracleTmp)
	if err == nil {
		return &gasPriceOracleTmp, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(GAS_PRICE_ORACLE, ""), nil, nil,
	)
	if err != nil {
		client.logger.Errorf("error getting gas price oracle from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var gasPriceOracle GasPriceOracle
	if err := json.Unmarshal(respBody.Bytes(), &gasPriceOracle); err != nil {
		client.logger.Errorf("error parsing gas price oracle from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, gasPriceOracle, 10*60*1000*time.Millisecond)

	return &gasPriceOracle, nil
}

func (client *HTTPClient) AddressCoinBalanceHistoryChart(addressHash string) ([]CoinBalancesByDate, error) {
	cacheKey := fmt.Sprintf("BlockscoutAddressCoinBalanceHistoryChart_%s", addressHash)
	var coinBalancesByDateTmp []CoinBalancesByDate

	err := client.httpCache.Get(cacheKey, &coinBalancesByDateTmp)
	if err == nil {
		return coinBalancesByDateTmp, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(strings.ReplaceAll(ADDRESS_COIN_BALANCE_HISTORY_CHART, "{addresshash}", addressHash), ""), nil, nil,
	)
	if err != nil {
		client.logger.Errorf("error getting address coin balance history from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var coinBalancesByDate []CoinBalancesByDate
	if err := json.Unmarshal(respBody.Bytes(), &coinBalancesByDate); err != nil {
		client.logger.Errorf("error parsing address coin balance history from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, coinBalancesByDate, 10*60*1000*time.Millisecond)

	return coinBalancesByDate, nil
}

func (client *HTTPClient) GetRawTraceByTxHash(evmTxHash string) (interface{}, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetRawTraceByTxHash_%s", evmTxHash)
	var rawTraceTmp CommonResp

	err := client.httpCache.Get(cacheKey, &rawTraceTmp)
	if err == nil {
		return &rawTraceTmp.Result, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(GET_RAW_TRACE_BY_TX_HASH, evmTxHash), nil, nil,
	)
	if err != nil {
		return nil, err
	}
	defer rawRespBody.Close()

	var rawTrace CommonResp
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&rawTrace); err != nil {
		return nil, err
	}

	if rawTrace.Status == "0" {
		return nil, fmt.Errorf(TX_NOT_FOUND)
	}

	client.httpCache.Set(cacheKey, &rawTrace, 10*60*1000*time.Millisecond)

	return &rawTrace.Result, nil
}

func (client *HTTPClient) GetListTokensOfAnAddress(addressHash string, queryParams []string, mappingParams map[string]string) (*TokensAddressResp, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetListTokensOfAnAddress_%s_%s_%s", addressHash, mappingParams["page"], mappingParams["offset"])
	var tokensAddressRespTmp TokensAddressResp

	err := client.httpCache.Get(cacheKey, &tokensAddressRespTmp)
	if err == nil {
		return &tokensAddressRespTmp, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(GET_LIST_TOKEN_OF_AN_ADDRESS, addressHash), queryParams, mappingParams,
	)
	if err != nil {
		client.logger.Errorf("error getting list tokens of an address from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var tokensAddressResp TokensAddressResp
	if err := json.Unmarshal(respBody.Bytes(), &tokensAddressResp); err != nil {
		client.logger.Errorf("error parsing list token of an address from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, &tokensAddressResp, 60*1000*time.Millisecond)

	return &tokensAddressResp, nil
}

func (client *HTTPClient) GetAddressCoinBalanceHistory(addressHash string, queryParams []string, mappingParams map[string]string) (*CommonPaginationResp, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetAddressCoinBalanceHistory_%s_%s_%s", addressHash, mappingParams["page"], mappingParams["offset"])
	var commonRespTmp CommonPaginationResp

	err := client.httpCache.Get(cacheKey, &commonRespTmp)
	if err == nil {
		return &commonRespTmp, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(GET_ADDRESS_COIN_BALANCE_HISTORY, addressHash), queryParams, mappingParams,
	)
	if err != nil {
		client.logger.Errorf("error getting address coin balances history from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var commonResp CommonPaginationResp
	if err := json.Unmarshal(respBody.Bytes(), &commonResp); err != nil {
		client.logger.Errorf("error parsing address coin balances history from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, &commonResp, 60*1000*time.Millisecond)

	return &commonResp, nil
}

func (client *HTTPClient) GetListInternalTxsByAddressHash(addressHash string, queryParams []string, mappingParams map[string]string) (*CommonPaginationResp, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetListInternalTxsByAddressHash_%s_%s_%s", addressHash, mappingParams["page"], mappingParams["offset"])
	var commonRespTmp CommonPaginationResp

	err := client.httpCache.Get(cacheKey, &commonRespTmp)
	if err == nil {
		return &commonRespTmp, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(GET_LIST_INTERNAL_TXS_BY_ADDRESS_HASH, addressHash), queryParams, mappingParams,
	)
	if err != nil {
		client.logger.Errorf("error getting list internal txs by address hash from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var commonResp CommonPaginationResp
	if err := json.Unmarshal(respBody.Bytes(), &commonResp); err != nil {
		client.logger.Errorf("error parsing list internal txs by address hash from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, &commonResp, 10*60*1000*time.Millisecond)

	return &commonResp, nil
}
