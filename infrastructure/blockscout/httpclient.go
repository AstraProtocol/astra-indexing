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
	utils "github.com/AstraProtocol/astra-indexing/infrastructure"
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
const EVM_VERSIONS = "/api/v1/evm-versions"
const COMPILER_VERSIONS = "/api/v1/compiler-versions?compiler="
const ADDRESS_COIN_BALANCE_HISTORY_CHART = "/address/{addresshash}/coin-balances/by-day?type=JSON"
const GET_RAW_TRACE_BY_TX_HASH = "/api/v1?module=transaction&action=getrawtracebytxhash&txhash="
const GET_LIST_TOKEN_OF_AN_ADDRESS = "/api/v1?module=account&action=tokenlist&address="
const GET_ADDRESS_COIN_BALANCE_HISTORY = "/api/v1?module=account&action=getcoinbalancehistory&address="
const GET_LIST_INTERNAL_TXS_BY_ADDRESS_HASH = "/api/v1?module=account&action=txlistinternal&address="
const GET_LIST_TOKEN_TRANSFERS_BY_ADDRESS_HASH = "/api/v1?module=account&action=getlisttokentransfers&address="
const GET_LIST_TOKEN_TRANSFERS_BY_CONTRACT_ADDRESS_HASH = "/api/v1?module=token&action=getlisttokentransfers&contractaddress="
const GET_LIST_TXS_BY_CONTRACT_ADDRESS_HASH = "/api/v1?module=account&action=txlist&address="
const GET_LIST_DEPOSIT_TXS_BY_CONTRACT_ADDRESS_HASH = "/api/v1?module=account&action=txlistdeposit&address="
const GET_TOKENS_HOLDER_OF_A_CONTRACT_ADDRESS = "/api/v1?module=token&action=getTokenHolders&contractaddress="
const GET_TOKEN_INVENTORY = "/api/v1?module=token&action=getinventory&contractaddress="
const GET_TOKEN_TRANSFERS_BY_TOKEN_ID = "/api/v1?module=token&action=tokentransfersbytokenid&contractaddress={contractaddresshash}&tokenid={tokenid}"
const GET_SOURCE_CODE = "/api/v1?module=contract&action=getsourcecode&address="
const GET_TOKEN_DETAIL = "/api/v1?module=token&action=gettoken&contractaddress="
const GET_TOKEN_METADATA = "/api/v1?module=token&action=getmetadata&contractaddress={contractaddresshash}&tokenid={tokenid}"
const UPDATE_ADDRESS_BALANCE = "/api/v1?module=account&action=update_balance&address={addresshash}&block={blockheight}&balance={balance}"
const HARD_HAT_POST_INTERFACE = "/api"
const VERIFY_FLATTENED = "/verify_smart_contract/contract_verifications"
const CHECK_VERIFY_STATUS = "/api/v1?module=contract&action=checkverifystatus&guid="
const GET_SOURCE_CODE_HARD_HAT_INTERFACE = "/api?module=contract&action=getsourcecode&address="
const GET_LIST_TXS_WITH_TOKEN_TRANSFERS_BY_TX_HASHES = "/api/v1?module=transaction&action=gettxswithtokentransfersbytxhashes&txhash="
const TX_NOT_FOUND = "transaction not found"
const ADDRESS_NOT_FOUND = "address not found"
const BALANCE_UPDATE_FAILED = "balance update failed"
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
		prometheus.RecordApiExecTime(queryUrl, strconv.Itoa(408), "http", time.Since(startTime).Milliseconds())
		return nil, fmt.Errorf("error creating HTTP request with context: %v", err)
	}
	rawResp, err := client.httpClient.Do(req)
	if err != nil {
		prometheus.RecordApiExecTime(queryUrl, strconv.Itoa(400), "http", time.Since(startTime).Milliseconds())
		return nil, fmt.Errorf("error requesting blockscout %s endpoint: %v", queryUrl, err)
	}

	prometheus.RecordApiExecTime(queryUrl, strconv.Itoa(rawResp.StatusCode), "http", time.Since(startTime).Milliseconds())

	if rawResp.StatusCode != 200 {
		rawResp.Body.Close()
		return nil, fmt.Errorf("error requesting blockscout %s endpoint: %s", queryUrl, rawResp.Status)
	}

	return rawResp.Body, nil
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
			"module": "BlockscoutHttpClient",
		}),
		httpClient,
		strings.TrimSuffix(url, "/"),
		cache.NewCache(),
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

	client.httpCache.Set(cacheKey, &txResp.Result, utils.TIME_CACHE_MEDIUM)

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

	client.httpCache.Set(cacheKey, &txResp.Result, utils.TIME_CACHE_MEDIUM)

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

	client.httpCache.Set(cacheKey, transactionEvmResp, utils.TIME_CACHE_LONG)

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

	client.httpCache.Set(cacheKey, transactionEvmResp, utils.TIME_CACHE_LONG)

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

	client.httpCache.Set(cacheKey, commonStats, utils.TIME_CACHE_LONG)

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

	client.httpCache.Set(cacheKey, addressCounterResp, utils.TIME_CACHE_MEDIUM)

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

	client.httpCache.Set(cacheKey, addressResp, utils.TIME_CACHE_FAST)

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

	client.httpCache.Set(cacheKey, seachResults, utils.TIME_CACHE_FAST)

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

	client.httpCache.Set(cacheKey, seachResults, utils.TIME_CACHE_FAST)

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

	client.httpCache.Set(cacheKey, &topAddressesBalanceResp, utils.TIME_CACHE_FAST)

	return &topAddressesBalanceResp, nil
}

func (client *HTTPClient) EthBlockNumber() (*EthBlockNumber, error) {
	cacheKey := "EthBlockNumber"
	var ethBlockNumber EthBlockNumber

	err := client.httpCache.Get(cacheKey, &ethBlockNumber)
	if err == nil {
		return &ethBlockNumber, nil
	}
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

	if err := json.Unmarshal(respBody.Bytes(), &ethBlockNumber); err != nil {
		client.logger.Errorf("error parsing eth block number from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, &ethBlockNumber, 2*time.Second)
	return &ethBlockNumber, nil
}

func (client *HTTPClient) GetListTokens(queryParams []string, mappingParams map[string]string) (*CommonPaginationResp, error) {
	cacheKey := fmt.Sprintf("BlockscouGetListTokens_%s_%s", mappingParams["page"], mappingParams["offset"])
	var commonPaginationRespTmp CommonPaginationResp

	err := client.httpCache.Get(cacheKey, &commonPaginationRespTmp)
	if err == nil {
		return &commonPaginationRespTmp, nil
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

	var commonPaginationResp CommonPaginationResp
	if err := json.Unmarshal(respBody.Bytes(), &commonPaginationResp); err != nil {
		client.logger.Errorf("error parsing list token from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, &commonPaginationResp, utils.TIME_CACHE_MEDIUM)

	return &commonPaginationResp, nil
}

func (client *HTTPClient) GetListInternalTxs(evmTxHash string) ([]InternalTransaction, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetListInternalTxs_%s", evmTxHash)
	var internalTxsTmp InternalTxsResp

	err := client.httpCache.Get(cacheKey, &internalTxsTmp)
	if err == nil {
		return internalTxsTmp.Result, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(GET_LIST_INTERNAL_TXS_BY_EVM_TX_HASH, evmTxHash), nil, nil,
	)
	if err != nil {
		client.logger.Errorf("error getting list internal txs by tx hash from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var internalTxsResp InternalTxsResp
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&internalTxsResp); err != nil {
		client.logger.Errorf("error parsing list internal txs by address hash from blockscout: %v", err)
		return nil, err
	}

	client.httpCache.Set(cacheKey, internalTxsResp, utils.TIME_CACHE_MEDIUM)

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

	client.httpCache.Set(cacheKey, abiResp.Result, utils.TIME_CACHE_LONG)

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

	client.httpCache.Set(cacheKey, &abiResp.Result, utils.TIME_CACHE_LONG)

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

	client.httpCache.Set(cacheKey, marketHistory, utils.TIME_CACHE_LONG)

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

	client.httpCache.Set(cacheKey, gasPriceOracle, utils.TIME_CACHE_LONG)

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

	client.httpCache.Set(cacheKey, coinBalancesByDate, utils.TIME_CACHE_MEDIUM)

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

	client.httpCache.Set(cacheKey, &rawTrace, utils.TIME_CACHE_LONG)

	return &rawTrace.Result, nil
}

func (client *HTTPClient) GetListTokensOfAnAddress(addressHash string, queryParams []string, mappingParams map[string]string) (*TokensAddressResp, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetListTokensOfAnAddress_%s_%s_%s_%s", addressHash, mappingParams["type"], mappingParams["page"], mappingParams["limit"])
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

	client.httpCache.Set(cacheKey, &tokensAddressResp, utils.TIME_CACHE_MEDIUM)

	return &tokensAddressResp, nil
}

func (client *HTTPClient) GetAddressCoinBalanceHistory(addressHash string, queryParams []string, mappingParams map[string]string) (*CommonPaginationPathResp, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetAddressCoinBalanceHistory_%s_%s_%s", addressHash, mappingParams["page"], mappingParams["offset"])
	var commonRespTmp CommonPaginationPathResp

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

	var commonResp CommonPaginationPathResp
	if err := json.Unmarshal(respBody.Bytes(), &commonResp); err != nil {
		client.logger.Errorf("error parsing address coin balances history from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, &commonResp, utils.TIME_CACHE_MEDIUM)

	return &commonResp, nil
}

func (client *HTTPClient) GetListInternalTxsByAddressHash(addressHash string, queryParams []string, mappingParams map[string]string) (*CommonPaginationPathResp, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetListInternalTxsByAddressHash_%s_%s_%s", addressHash, mappingParams["page"], mappingParams["offset"])
	var commonRespTmp CommonPaginationPathResp

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

	var commonResp CommonPaginationPathResp
	if err := json.Unmarshal(respBody.Bytes(), &commonResp); err != nil {
		client.logger.Errorf("error parsing list internal txs by address hash from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, &commonResp, utils.TIME_CACHE_MEDIUM)

	return &commonResp, nil
}

func (client *HTTPClient) GetListInternalTxsByTxHash(txHash string, queryParams []string, mappingParams map[string]string) (*CommonPaginationPathResp, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetListInternalTxsByTxHash_%s_%s_%s", txHash, mappingParams["page"], mappingParams["offset"])
	var commonRespTmp CommonPaginationPathResp

	err := client.httpCache.Get(cacheKey, &commonRespTmp)
	if err == nil {
		return &commonRespTmp, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(GET_LIST_INTERNAL_TXS_BY_EVM_TX_HASH, txHash), queryParams, mappingParams,
	)
	if err != nil {
		client.logger.Errorf("error getting list internal txs by tx hash from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var commonResp CommonPaginationPathResp
	if err := json.Unmarshal(respBody.Bytes(), &commonResp); err != nil {
		client.logger.Errorf("error parsing list internal txs by tx hash from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, &commonResp, utils.TIME_CACHE_MEDIUM)

	return &commonResp, nil
}

func (client *HTTPClient) GetListTokenTransfersByAddressHash(addressHash string, queryParams []string, mappingParams map[string]string) (*CommonPaginationPathResp, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetListTokenTransfersByAddressHash_%s_%s_%s", addressHash, mappingParams["page"], mappingParams["offset"])
	var commonPaginationRespTmp CommonPaginationPathResp

	err := client.httpCache.Get(cacheKey, &commonPaginationRespTmp)
	if err == nil {
		return &commonPaginationRespTmp, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(GET_LIST_TOKEN_TRANSFERS_BY_ADDRESS_HASH, addressHash), queryParams, mappingParams,
	)
	if err != nil {
		client.logger.Errorf("error getting list token transfers by address hash from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var commonPaginationResp CommonPaginationPathResp
	if err := json.Unmarshal(respBody.Bytes(), &commonPaginationResp); err != nil {
		client.logger.Errorf("error parsing list token transfers by address hash from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, &commonPaginationResp, utils.TIME_CACHE_MEDIUM)

	return &commonPaginationResp, nil
}

func (client *HTTPClient) GetListTokenTransfersByContractAddressHash(contractAddressHash string, queryParams []string, mappingParams map[string]string) (*CommonPaginationPathResp, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetListTokenTransfersByContractAddressHash_%s_%s_%s", contractAddressHash, mappingParams["page"], mappingParams["offset"])
	var commonPaginationRespTmp CommonPaginationPathResp

	err := client.httpCache.Get(cacheKey, &commonPaginationRespTmp)
	if err == nil {
		return &commonPaginationRespTmp, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(GET_LIST_TOKEN_TRANSFERS_BY_CONTRACT_ADDRESS_HASH, contractAddressHash), queryParams, mappingParams,
	)
	if err != nil {
		client.logger.Errorf("error getting list token transfers by contract address hash from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var commonPaginationResp CommonPaginationPathResp
	if err := json.Unmarshal(respBody.Bytes(), &commonPaginationResp); err != nil {
		client.logger.Errorf("error parsing list token transfers by contract address hash from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, &commonPaginationResp, utils.TIME_CACHE_MEDIUM)

	return &commonPaginationResp, nil
}

func (client *HTTPClient) GetListTxsByContractAddressHash(contractAddressHash string, queryParams []string, mappingParams map[string]string) (*CommonPaginationPathResp, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetListTxsByContractAddressHash_%s_%s_%s_%s", contractAddressHash, mappingParams["page"], mappingParams["offset"], mappingParams["filter"])
	var commonPaginationRespTmp CommonPaginationPathResp

	err := client.httpCache.Get(cacheKey, &commonPaginationRespTmp)
	if err == nil {
		return &commonPaginationRespTmp, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(GET_LIST_TXS_BY_CONTRACT_ADDRESS_HASH, contractAddressHash), queryParams, mappingParams,
	)
	if err != nil {
		client.logger.Errorf("error getting list txs by contract address hash from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var commonPaginationResp CommonPaginationPathResp
	if err := json.Unmarshal(respBody.Bytes(), &commonPaginationResp); err != nil {
		client.logger.Errorf("error parsing list txs by contract address hash from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, &commonPaginationResp, utils.TIME_CACHE_FAST)

	return &commonPaginationResp, nil
}

func (client *HTTPClient) GetListDepositTxsByContractAddressHash(contractAddressHash string, queryParams []string, mappingParams map[string]string) (*CommonPaginationPathResp, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetListDepositTxsByContractAddressHash_%s_%s_%s", contractAddressHash, mappingParams["page"], mappingParams["offset"])
	var commonPaginationRespTmp CommonPaginationPathResp

	err := client.httpCache.Get(cacheKey, &commonPaginationRespTmp)
	if err == nil {
		return &commonPaginationRespTmp, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(GET_LIST_DEPOSIT_TXS_BY_CONTRACT_ADDRESS_HASH, contractAddressHash), queryParams, mappingParams,
	)
	if err != nil {
		client.logger.Errorf("error getting list deposit txs by contract address hash from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var commonPaginationResp CommonPaginationPathResp
	if err := json.Unmarshal(respBody.Bytes(), &commonPaginationResp); err != nil {
		client.logger.Errorf("error parsing list deposit txs by contract address hash from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, &commonPaginationResp, utils.TIME_CACHE_FAST)

	return &commonPaginationResp, nil
}

func (client *HTTPClient) GetTokenHoldersOfAContractAddress(contractAddressHash string, queryParams []string, mappingParams map[string]string) (*CommonPaginationResp, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetTokenHoldersOfAContractAddressHash_%s_%s_%s", contractAddressHash, mappingParams["page"], mappingParams["offset"])
	var commonPaginationRespTmp CommonPaginationResp

	err := client.httpCache.Get(cacheKey, &commonPaginationRespTmp)
	if err == nil {
		return &commonPaginationRespTmp, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(GET_TOKENS_HOLDER_OF_A_CONTRACT_ADDRESS, contractAddressHash), queryParams, mappingParams,
	)
	if err != nil {
		client.logger.Errorf("error getting list token holders of a contract address hash from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var commonPaginationResp CommonPaginationResp
	if err := json.Unmarshal(respBody.Bytes(), &commonPaginationResp); err != nil {
		client.logger.Errorf("error parsing list token holders of a contract address hash from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, &commonPaginationResp, utils.TIME_CACHE_MEDIUM)

	return &commonPaginationResp, nil
}

func (client *HTTPClient) GetTokenInventoryOfAContractAddress(contractAddressHash string, queryParams []string, mappingParams map[string]string) (*CommonPaginationPathResp, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetTokenInventoryOfAContractAddress_%s_%s_%s", contractAddressHash, mappingParams["page"], mappingParams["offset"])
	var commonPaginationRespTmp CommonPaginationPathResp

	err := client.httpCache.Get(cacheKey, &commonPaginationRespTmp)
	if err == nil {
		return &commonPaginationRespTmp, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(GET_TOKEN_INVENTORY, contractAddressHash), queryParams, mappingParams,
	)
	if err != nil {
		client.logger.Errorf("error getting token inventory of a contract address hash from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var commonPaginationResp CommonPaginationPathResp
	if err := json.Unmarshal(respBody.Bytes(), &commonPaginationResp); err != nil {
		client.logger.Errorf("error parsing token inventory of a contract address hash from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, &commonPaginationResp, utils.TIME_CACHE_MEDIUM)

	return &commonPaginationResp, nil
}

func (client *HTTPClient) GetTokenTransfersByTokenId(contractAddressHash string, tokenId string, queryParams []string, mappingParams map[string]string) (*CommonPaginationPathResp, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetTokenTransfersByTokenId_%s_%s_%s_%s", contractAddressHash, tokenId, mappingParams["page"], mappingParams["offset"])
	var commonPaginationRespTmp CommonPaginationPathResp

	err := client.httpCache.Get(cacheKey, &commonPaginationRespTmp)
	if err == nil {
		return &commonPaginationRespTmp, nil
	}

	url := strings.ReplaceAll(GET_TOKEN_TRANSFERS_BY_TOKEN_ID, "{contractaddresshash}", contractAddressHash)
	rawRespBody, err := client.request(
		client.getUrl(strings.ReplaceAll(url, "{tokenid}", tokenId), ""), queryParams, mappingParams,
	)
	if err != nil {
		client.logger.Errorf("error getting token transfers by token id from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var commonPaginationResp CommonPaginationPathResp
	if err := json.Unmarshal(respBody.Bytes(), &commonPaginationResp); err != nil {
		client.logger.Errorf("error parsing token transfers by token id from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, &commonPaginationResp, utils.TIME_CACHE_MEDIUM)

	return &commonPaginationResp, nil
}

func (client *HTTPClient) GetSourceCodeByContractAddressHash(contractAddressHash string) (interface{}, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetSourceCodeByContractAddressHash_%s", contractAddressHash)
	var commonRespTmp CommonResp

	err := client.httpCache.Get(cacheKey, &commonRespTmp)
	if err == nil {
		return &commonRespTmp.Result, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(GET_SOURCE_CODE, contractAddressHash), nil, nil,
	)
	if err != nil {
		return nil, err
	}
	defer rawRespBody.Close()

	var commonResp CommonResp
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&commonResp); err != nil {
		return nil, err
	}

	if commonResp.Status == "0" {
		return nil, fmt.Errorf(ADDRESS_NOT_FOUND)
	}

	client.httpCache.Set(cacheKey, &commonResp, utils.TIME_CACHE_MEDIUM)

	return &commonResp.Result, nil
}

func (client *HTTPClient) EvmVersions() (interface{}, error) {
	cacheKey := "BlockscoutEvmVersions"
	var commonRespTmp CommonResp

	err := client.httpCache.Get(cacheKey, &commonRespTmp)
	if err == nil {
		return &commonRespTmp.Result, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(EVM_VERSIONS, ""), nil, nil,
	)
	if err != nil {
		client.logger.Errorf("error getting evm versions from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var commonResp CommonResp
	if err := json.Unmarshal(respBody.Bytes(), &commonResp); err != nil {
		client.logger.Errorf("error parsing evm versions from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, commonResp, utils.TIME_CACHE_LONG)

	return &commonResp.Result, nil
}

func (client *HTTPClient) CompilerVersions(compiler string) (interface{}, error) {
	cacheKey := fmt.Sprintf("BlockscoutCompilerVersions_%s", compiler)
	var commonRespTmp CommonResp

	err := client.httpCache.Get(cacheKey, &commonRespTmp)
	if err == nil {
		return &commonRespTmp.Result, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(COMPILER_VERSIONS, compiler), nil, nil,
	)
	if err != nil {
		client.logger.Errorf("error getting compiler versions from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var commonResp CommonResp
	if err := json.Unmarshal(respBody.Bytes(), &commonResp); err != nil {
		client.logger.Errorf("error parsing compiler versions from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, commonResp, utils.TIME_CACHE_LONG)

	return &commonResp.Result, nil
}

func (client *HTTPClient) GetTokenDetail(contractAddressHash string) (interface{}, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetTokenbDetail_%s", contractAddressHash)
	var commonRespTmp CommonResp

	err := client.httpCache.Get(cacheKey, &commonRespTmp)
	if err == nil {
		return commonRespTmp.Result, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(GET_TOKEN_DETAIL, contractAddressHash), nil, nil,
	)
	if err != nil {
		client.logger.Errorf("error getting token detail by contract address hash from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var commonResp CommonResp
	if err := json.Unmarshal(respBody.Bytes(), &commonResp); err != nil {
		client.logger.Errorf("error parsing token detail by contract address hash from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, &commonResp, utils.TIME_CACHE_MEDIUM)

	return commonResp.Result, nil
}

func (client *HTTPClient) GetTokenMetadata(contractAddressHash string, tokenId string) (interface{}, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetTokenMetadata_%s_%s", contractAddressHash, tokenId)
	var commonRespTmp CommonResp

	err := client.httpCache.Get(cacheKey, &commonRespTmp)
	if err == nil {
		return commonRespTmp.Result, nil
	}

	url := strings.ReplaceAll(GET_TOKEN_METADATA, "{contractaddresshash}", contractAddressHash)
	rawRespBody, err := client.request(
		client.getUrl(strings.ReplaceAll(url, "{tokenid}", tokenId), ""), nil, nil,
	)
	if err != nil {
		client.logger.Errorf("error getting token metadata from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var commonResp CommonResp
	if err := json.Unmarshal(respBody.Bytes(), &commonResp); err != nil {
		client.logger.Errorf("error parsing token metadata from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, commonResp, utils.TIME_CACHE_MEDIUM)

	return commonResp.Result, nil
}

func (client *HTTPClient) UpdateAddressBalance(addressHash string, blockHeight string, balance string) (interface{}, error) {
	cacheKey := fmt.Sprintf("BlockscoutUpdateAddressBalance_%s", addressHash)
	var commonRespTmp CommonResp

	err := client.httpCache.Get(cacheKey, &commonRespTmp)
	if err == nil {
		return commonRespTmp.Result, nil
	}

	url := strings.ReplaceAll(UPDATE_ADDRESS_BALANCE, "{addresshash}", addressHash)
	url = strings.ReplaceAll(url, "{blockheight}", blockHeight)
	rawRespBody, err := client.request(
		client.getUrl(strings.ReplaceAll(url, "{balance}", balance), ""), nil, nil,
	)

	if err != nil {
		return nil, err
	}
	defer rawRespBody.Close()

	var commonResp CommonResp
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&commonResp); err != nil {
		return nil, err
	}

	if commonResp.Status == "0" {
		return "", fmt.Errorf("BALANCE_UPDATE_FAILED")
	}

	client.httpCache.Set(cacheKey, commonResp, utils.TIME_CACHE_FAST)

	return commonResp.Result, nil
}

func (client *HTTPClient) Verify(bodyParams interface{}) (interface{}, error) {
	m, ok := bodyParams.(map[string]string)
	if !ok {
		return nil, fmt.Errorf("Verify: cannot convert rawBody to map")
	}

	module := m["module"]
	action := m["action"]
	codeFormat := m["codeformat"]
	contractAddress := m["contractaddress"]
	cacheKey := fmt.Sprintf("BlockscoutVerify_%s_%s_%s_%s", module, action, codeFormat, contractAddress)

	var commonRespTmp CommonResp
	err := client.httpCache.Get(cacheKey, &commonRespTmp)
	if err == nil {
		return commonRespTmp, nil
	}

	postBody, err := json.Marshal(bodyParams)
	if err != nil {
		return nil, err
	}

	rawRespBody, err := client.requestPost(client.getUrl(HARD_HAT_POST_INTERFACE, ""), postBody)
	if err != nil {
		return nil, err
	}
	defer rawRespBody.Close()

	var commonResp CommonResp
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&commonResp); err != nil {
		return nil, err
	}

	if commonResp.Status == "0" {
		return nil, fmt.Errorf("Verify: %s", commonResp.Message)
	}

	client.httpCache.Set(cacheKey, commonResp, utils.TIME_CACHE_MEDIUM)

	return commonResp, nil
}

func (client *HTTPClient) VerifyFlattened(bodyParams interface{}) (interface{}, error) {
	m, ok := bodyParams.(map[string](map[string]string))
	if !ok {
		return nil, fmt.Errorf("VerifyFlattened: cannot convert rawBody to map")
	}

	smartContractParams := m["smart_contract"]
	cacheKey := fmt.Sprintf("BlockscoutVerifyFlattened_%s_%s", smartContractParams["address_hash"], smartContractParams["name"])

	var respTmp interface{}
	err := client.httpCache.Get(cacheKey, &respTmp)
	if err == nil {
		return respTmp, nil
	}

	postBody, err := json.Marshal(bodyParams)
	if err != nil {
		return nil, err
	}

	rawRespBody, err := client.requestPost(client.getUrl(VERIFY_FLATTENED, ""), postBody)
	if err != nil {
		return nil, err
	}
	defer rawRespBody.Close()

	var resp interface{}
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&resp); err != nil {
		return nil, err
	}

	client.httpCache.Set(cacheKey, resp, utils.TIME_CACHE_MEDIUM)

	return resp, nil
}

func (client *HTTPClient) CheckVerifyStatus(guid string) (interface{}, error) {
	cacheKey := fmt.Sprintf("BlockscoutCheckVerifyStatus_%s", guid)
	var respTmp interface{}

	err := client.httpCache.Get(cacheKey, &respTmp)
	if err == nil {
		return respTmp, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(CHECK_VERIFY_STATUS, guid), nil, nil,
	)
	if err != nil {
		client.logger.Errorf("error checking verify status by guid from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var resp interface{}
	if err := json.Unmarshal(respBody.Bytes(), &resp); err != nil {
		client.logger.Errorf("error parsing verify status by guid from from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, &resp, utils.TIME_CACHE_MEDIUM)

	return resp, nil
}

func (client *HTTPClient) GetSourceCode(addressHash string) (*SourceCodeResp, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetSourceCode_%s", addressHash)
	var respTmp SourceCodeResp

	err := client.httpCache.Get(cacheKey, &respTmp)
	if err == nil {
		return &respTmp, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(GET_SOURCE_CODE_HARD_HAT_INTERFACE, addressHash), nil, nil,
	)
	if err != nil {
		client.logger.Errorf("error get source code by address hash from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var resp SourceCodeResp
	if err := json.Unmarshal(respBody.Bytes(), &resp); err != nil {
		client.logger.Errorf("error parsing source code by address hash from from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, &resp, utils.TIME_CACHE_MEDIUM)

	return &resp, nil
}

func (client *HTTPClient) Logs(bodyParams interface{}) (interface{}, error) {
	m, ok := bodyParams.(map[string]string)
	if !ok {
		return nil, fmt.Errorf("Logs: cannot convert rawBody to map")
	}

	module := m["module"]
	action := m["action"]
	fromBlock := m["fromBlock"]
	toBlock := m["toBlock"]
	address := m["address"]
	topic0 := m["topic0"]
	cacheKey := fmt.Sprintf("BlockscoutLogs_%s_%s_%s_%s_%s_%s", module, action, fromBlock, toBlock, address, topic0)

	var commonRespTmp CommonResp
	err := client.httpCache.Get(cacheKey, &commonRespTmp)
	if err == nil {
		return commonRespTmp, nil
	}

	postBody, err := json.Marshal(bodyParams)
	if err != nil {
		return nil, err
	}

	rawRespBody, err := client.requestPost(client.getUrl(HARD_HAT_POST_INTERFACE, ""), postBody)
	if err != nil {
		return nil, err
	}
	defer rawRespBody.Close()

	var commonResp CommonResp
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&commonResp); err != nil {
		return nil, err
	}

	if commonResp.Status == "0" {
		return nil, fmt.Errorf("Logs: %s", commonResp.Message)
	}

	client.httpCache.Set(cacheKey, commonResp, utils.TIME_CACHE_MEDIUM)

	return commonResp, nil
}

func (client *HTTPClient) GetListTxsWithTokenTransfersByTxHashes(txHashes string) (*TxsResp, error) {
	cacheKey := fmt.Sprintf("BlockscoutGetListTxsWithTokenTransfersByTxHashes_%s", txHashes)
	var txsRespTmp TxsResp

	err := client.httpCache.Get(cacheKey, &txsRespTmp)
	if err == nil {
		return &txsRespTmp, nil
	}

	rawRespBody, err := client.request(
		client.getUrl(GET_LIST_TXS_WITH_TOKEN_TRANSFERS_BY_TX_HASHES, txHashes), nil, nil,
	)
	if err != nil {
		client.logger.Errorf("error getting list tx with token transfers by tx hashes from blockscout: %v", err)
		return nil, err
	}
	defer rawRespBody.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(rawRespBody)

	var txsResp TxsResp
	if err := json.Unmarshal(respBody.Bytes(), &txsResp); err != nil {
		client.logger.Errorf("error parsing list tx with token transfers by tx hashes from blockscout: %v", err)
	}

	client.httpCache.Set(cacheKey, &txsResp, utils.TIME_CACHE_FAST)

	return &txsResp, nil
}
