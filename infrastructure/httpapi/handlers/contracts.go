package handlers

import (
	"errors"
	"strconv"
	"time"

	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	blockscout_infrastructure "github.com/AstraProtocol/astra-indexing/infrastructure/blockscout"
	"github.com/AstraProtocol/astra-indexing/infrastructure/httpapi"
	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"
	"github.com/valyala/fasthttp"
)

type Contracts struct {
	logger           applogger.Logger
	blockscoutClient blockscout_infrastructure.HTTPClient
}

func NewContracts(
	logger applogger.Logger,
	blockscoutClient blockscout_infrastructure.HTTPClient,
) *Contracts {
	return &Contracts{
		logger.WithFields(applogger.LogFields{
			"module": "ContractsHandler",
		}),
		blockscoutClient,
	}
}

func (handler *Contracts) GetListTokens(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetListTokens"
	// handle api's params
	var err error
	var page int64
	var offset int64
	page = blockscout_infrastructure.DEFAULT_PAGE
	offset = blockscout_infrastructure.DEFAULT_OFFSET

	queryParams := make([]string, 0)
	mappingParams := make(map[string]string)

	if string(ctx.QueryArgs().Peek("blockscout")) != "true" {
		handler.logger.Errorf("invalid %s blockscout param", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid blockscout param"))
		return
	}

	if string(ctx.QueryArgs().Peek("page")) != "" {
		page, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("page")), 10, 0)
		if err != nil || page <= 0 {
			handler.logger.Errorf("invalid %s page param", recordMethod)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("invalid page param"))
			return
		}
	}
	queryParams = append(queryParams, "page")
	mappingParams["page"] = strconv.FormatInt(page, 10)

	if string(ctx.QueryArgs().Peek("offset")) != "" {
		offset, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("offset")), 10, 0)
		if err != nil || offset <= 0 {
			handler.logger.Errorf("invalid %s offset param", recordMethod)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("invalid offset param"))
			return
		}
	}
	queryParams = append(queryParams, "offset")
	mappingParams["offset"] = strconv.FormatInt(offset, 10)
	//

	listTokensResp, err := handler.blockscoutClient.GetListTokens(queryParams, mappingParams)
	if err != nil {
		handler.logger.Errorf("error fetching list tokens from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, listTokensResp)
}

func (handler *Contracts) GetListTokenTransfersByContractAddressHash(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetListTokenTransfersByContractAddressHash"
	// handle api's params
	var err error
	var page int64
	var offset int64
	page = blockscout_infrastructure.DEFAULT_PAGE
	offset = blockscout_infrastructure.DEFAULT_OFFSET

	queryParams := make([]string, 0)
	mappingParams := make(map[string]string)

	addressHash, contractParamOk := URLValueGuard(ctx, handler.logger, "contractaddress")
	if !contractParamOk {
		handler.logger.Errorf("invalid %s params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid contract param"))
		return
	}

	if string(ctx.QueryArgs().Peek("blockscout")) != "true" {
		handler.logger.Errorf("invalid %s blockscout param", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid blockscout param"))
		return
	}

	if string(ctx.QueryArgs().Peek("page")) != "" {
		page, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("page")), 10, 0)
		if err != nil || page <= 0 {
			handler.logger.Errorf("invalid %s page param", recordMethod)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("invalid page param"))
			return
		}
	}
	queryParams = append(queryParams, "page")
	mappingParams["page"] = strconv.FormatInt(page, 10)

	if string(ctx.QueryArgs().Peek("offset")) != "" {
		offset, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("offset")), 10, 0)
		if err != nil || offset <= 0 {
			handler.logger.Errorf("invalid %s offset param", recordMethod)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("invalid offset param"))
			return
		}
	}
	queryParams = append(queryParams, "offset")
	mappingParams["offset"] = strconv.FormatInt(offset, 10)

	if string(ctx.QueryArgs().Peek("block_number")) != "" {
		queryParams = append(queryParams, "block_number")
		mappingParams["block_number"] = string(ctx.QueryArgs().Peek("block_number"))
	}

	if string(ctx.QueryArgs().Peek("index")) != "" {
		queryParams = append(queryParams, "index")
		mappingParams["index"] = string(ctx.QueryArgs().Peek("index"))
	}
	//

	tokensAddressResp, err := handler.blockscoutClient.GetListTokenTransfersByContractAddressHash(addressHash, queryParams, mappingParams)
	if err != nil {
		handler.logger.Errorf("error fetching list token transfers by contract address from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, tokensAddressResp)
}

func (handler *Contracts) GetListTxsByContractAddressHash(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetListTxsByContractAddressHash"
	// handle api's params
	var err error
	var page int64
	var offset int64
	page = blockscout_infrastructure.DEFAULT_PAGE
	offset = blockscout_infrastructure.DEFAULT_OFFSET

	queryParams := make([]string, 0)
	mappingParams := make(map[string]string)

	addressHash, contractParamOk := URLValueGuard(ctx, handler.logger, "contractaddress")
	if !contractParamOk {
		handler.logger.Errorf("invalid %s params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid contractaddress param"))
		return
	}

	if string(ctx.QueryArgs().Peek("blockscout")) != "true" {
		handler.logger.Errorf("invalid %s blockscout param", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid blockscout param"))
		return
	}

	if string(ctx.QueryArgs().Peek("page")) != "" {
		page, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("page")), 10, 0)
		if err != nil || page <= 0 {
			handler.logger.Errorf("invalid %s page param", recordMethod)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("invalid page param"))
			return
		}
	}
	queryParams = append(queryParams, "page")
	mappingParams["page"] = strconv.FormatInt(page, 10)

	if string(ctx.QueryArgs().Peek("offset")) != "" {
		offset, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("offset")), 10, 0)
		if err != nil || offset <= 0 {
			handler.logger.Errorf("invalid %s offset param", recordMethod)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("invalid offset param"))
			return
		}
	}
	queryParams = append(queryParams, "offset")
	mappingParams["offset"] = strconv.FormatInt(offset, 10)

	if string(ctx.QueryArgs().Peek("block_number")) != "" {
		queryParams = append(queryParams, "block_number")
		mappingParams["block_number"] = string(ctx.QueryArgs().Peek("block_number"))
	}

	if string(ctx.QueryArgs().Peek("index")) != "" {
		queryParams = append(queryParams, "index")
		mappingParams["index"] = string(ctx.QueryArgs().Peek("index"))
	}
	//

	tokensAddressResp, err := handler.blockscoutClient.GetListTxsByContractAddressHash(addressHash, queryParams, mappingParams)
	if err != nil {
		handler.logger.Errorf("error fetching list txs of contract address from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, tokensAddressResp)
}

func (handler *Contracts) GetListTokenHoldersOfAContractAddressHash(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetListTokenHoldersOfAContractAddressHash"
	// handle api's params
	var err error
	var page int64
	var offset int64
	page = blockscout_infrastructure.DEFAULT_PAGE
	offset = blockscout_infrastructure.DEFAULT_OFFSET

	queryParams := make([]string, 0)
	mappingParams := make(map[string]string)

	addressHash, contractParamOk := URLValueGuard(ctx, handler.logger, "contractaddress")
	if !contractParamOk {
		handler.logger.Errorf("invalid %s params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid contractaddress param"))
		return
	}

	if string(ctx.QueryArgs().Peek("blockscout")) != "true" {
		handler.logger.Errorf("invalid %s blockscout param", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid blockscout param"))
		return
	}

	if string(ctx.QueryArgs().Peek("page")) != "" {
		page, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("page")), 10, 0)
		if err != nil || page <= 0 {
			handler.logger.Errorf("invalid %s page param", recordMethod)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("invalid page param"))
			return
		}
	}
	queryParams = append(queryParams, "page")
	mappingParams["page"] = strconv.FormatInt(page, 10)

	if string(ctx.QueryArgs().Peek("offset")) != "" {
		offset, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("offset")), 10, 0)
		if err != nil || offset <= 0 {
			handler.logger.Errorf("invalid %s offset param", recordMethod)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("invalid offset param"))
			return
		}
	}
	queryParams = append(queryParams, "offset")
	mappingParams["offset"] = strconv.FormatInt(offset, 10)
	//

	tokensAddressResp, err := handler.blockscoutClient.GetTokenHoldersOfAContractAddress(addressHash, queryParams, mappingParams)
	if err != nil {
		handler.logger.Errorf("error fetching token holders of a contract address from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, tokensAddressResp)
}

func (handler *Contracts) GetTokenInventoryOfAContractAddressHash(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetTokenInventoryOfAContractAddressHash"
	// handle api's params
	var err error
	var page int64
	var offset int64
	page = blockscout_infrastructure.DEFAULT_PAGE
	offset = blockscout_infrastructure.DEFAULT_OFFSET

	queryParams := make([]string, 0)
	mappingParams := make(map[string]string)

	addressHash, contractParamOk := URLValueGuard(ctx, handler.logger, "contractaddress")
	if !contractParamOk {
		handler.logger.Errorf("invalid %s params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid contractaddress param"))
		return
	}

	if string(ctx.QueryArgs().Peek("blockscout")) != "true" {
		handler.logger.Errorf("invalid %s blockscout param", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid blockscout param"))
		return
	}

	if string(ctx.QueryArgs().Peek("page")) != "" {
		page, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("page")), 10, 0)
		if err != nil || page <= 0 {
			handler.logger.Errorf("invalid %s page param", recordMethod)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("invalid page param"))
			return
		}
	}
	queryParams = append(queryParams, "page")
	mappingParams["page"] = strconv.FormatInt(page, 10)

	if string(ctx.QueryArgs().Peek("offset")) != "" {
		offset, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("offset")), 10, 0)
		if err != nil || offset <= 0 {
			handler.logger.Errorf("invalid %s offset param", recordMethod)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("invalid offset param"))
			return
		}
	}
	queryParams = append(queryParams, "offset")
	mappingParams["offset"] = strconv.FormatInt(offset, 10)

	if string(ctx.QueryArgs().Peek("token_id")) != "" {
		queryParams = append(queryParams, "token_id")
		mappingParams["token_id"] = string(ctx.QueryArgs().Peek("token_id"))
	}
	//

	tokensAddressResp, err := handler.blockscoutClient.GetTokenInventoryOfAContractAddress(addressHash, queryParams, mappingParams)
	if err != nil {
		handler.logger.Errorf("error fetching token inventory of a contract address from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, tokensAddressResp)
}

func (handler *Contracts) GetTokenTransfersByTokenId(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetTokenTransfersByTokenId"
	// handle api's params
	var err error
	var page int64
	var offset int64
	page = blockscout_infrastructure.DEFAULT_PAGE
	offset = blockscout_infrastructure.DEFAULT_OFFSET

	queryParams := make([]string, 0)
	mappingParams := make(map[string]string)

	addressHash, contractParamOk := URLValueGuard(ctx, handler.logger, "contractaddress")
	if !contractParamOk {
		handler.logger.Errorf("invalid %s address hash param", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid contractaddress param"))
		return
	}

	tokenId, tokenParamOk := URLValueGuard(ctx, handler.logger, "tokenid")
	if !tokenParamOk {
		handler.logger.Errorf("invalid %s token id param", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid tokenid param"))
		return
	}

	if string(ctx.QueryArgs().Peek("blockscout")) != "true" {
		handler.logger.Errorf("invalid %s blockscout param", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid blockscout param"))
		return
	}

	if string(ctx.QueryArgs().Peek("page")) != "" {
		page, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("page")), 10, 0)
		if err != nil || page <= 0 {
			handler.logger.Errorf("invalid %s page param", recordMethod)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("invalid page param"))
			return
		}
	}
	queryParams = append(queryParams, "page")
	mappingParams["page"] = strconv.FormatInt(page, 10)

	if string(ctx.QueryArgs().Peek("offset")) != "" {
		offset, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("offset")), 10, 0)
		if err != nil || offset <= 0 {
			handler.logger.Errorf("invalid %s offset param", recordMethod)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("invalid offset param"))
			return
		}
	}
	queryParams = append(queryParams, "offset")
	mappingParams["offset"] = strconv.FormatInt(offset, 10)

	if string(ctx.QueryArgs().Peek("block_number")) != "" {
		queryParams = append(queryParams, "block_number")
		mappingParams["block_number"] = string(ctx.QueryArgs().Peek("block_number"))
	}

	if string(ctx.QueryArgs().Peek("index")) != "" {
		queryParams = append(queryParams, "index")
		mappingParams["index"] = string(ctx.QueryArgs().Peek("index"))
	}
	//

	tokensAddressResp, err := handler.blockscoutClient.GetTokenTransfersByTokenId(addressHash, tokenId, queryParams, mappingParams)
	if err != nil {
		handler.logger.Errorf("error fetching token transfers by token id from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, tokensAddressResp)
}

func (handler *Contracts) GetSourceCodeOfAContractAddressHash(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetSourceCodeOfAContractAddressHash"
	// handle api's params
	var err error

	addressHash, contractParamOk := URLValueGuard(ctx, handler.logger, "contractaddress")
	if !contractParamOk {
		handler.logger.Errorf("invalid %s params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid contractaddress param"))
		return
	}
	//

	sourceCode, err := handler.blockscoutClient.GetSourceCodeByContractAddressHash(addressHash)
	if err != nil {
		handler.logger.Errorf("error fetching source code by contract address from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, sourceCode)
}

func (handler *Contracts) GetTokenDetail(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetTokenDetail"
	// handle api's params
	var err error

	addressHash, contractParamOk := URLValueGuard(ctx, handler.logger, "contractaddress")
	if !contractParamOk {
		handler.logger.Errorf("invalid %s params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid contractaddress param"))
		return
	}
	//

	tokenDetail, err := handler.blockscoutClient.GetTokenDetail(addressHash)
	if err != nil {
		handler.logger.Errorf("error fetching token detail from blockcscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, tokenDetail)
}

func (handler *Contracts) GetTokenMetadata(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetTokenMetadata"
	// handle api's params
	var err error

	addressHash, contractParamOk := URLValueGuard(ctx, handler.logger, "contractaddress")
	if !contractParamOk {
		handler.logger.Errorf("invalid %s address params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid contractaddress param"))
		return
	}

	tokenId, tokenParamOk := URLValueGuard(ctx, handler.logger, "tokenid")
	if !tokenParamOk {
		handler.logger.Errorf("invalid %s token id param", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid tokenid param"))
		return
	}
	//

	tokensAddressResp, err := handler.blockscoutClient.GetTokenMetadata(addressHash, tokenId)
	if err != nil {
		handler.logger.Errorf("error fetching token metadata from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, tokensAddressResp)
}
