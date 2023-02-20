package handlers

import (
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
		handler.logger.Error("invalid params")
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	if string(ctx.QueryArgs().Peek("page")) != "" {
		page, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("page")), 10, 0)
		if err != nil || page <= 0 {
			handler.logger.Error("page param is invalid")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
	}
	queryParams = append(queryParams, "page")
	mappingParams["page"] = strconv.FormatInt(page, 10)

	if string(ctx.QueryArgs().Peek("offset")) != "" {
		offset, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("offset")), 10, 0)
		if err != nil || offset <= 0 {
			handler.logger.Error("offset param is invalid")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
	}
	queryParams = append(queryParams, "offset")
	mappingParams["offset"] = strconv.FormatInt(offset, 10)
	//

	listTokensResp, err := handler.blockscoutClient.GetListTokens(queryParams, mappingParams)
	if err != nil {
		handler.logger.Errorf("error getting list tokens from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
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
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		return
	}

	if string(ctx.QueryArgs().Peek("blockscout")) != "true" {
		handler.logger.Error("invalid params")
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	if string(ctx.QueryArgs().Peek("page")) != "" {
		page, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("page")), 10, 0)
		if err != nil || page <= 0 {
			handler.logger.Error("page param is invalid")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
	}
	queryParams = append(queryParams, "page")
	mappingParams["page"] = strconv.FormatInt(page, 10)

	if string(ctx.QueryArgs().Peek("offset")) != "" {
		offset, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("offset")), 10, 0)
		if err != nil || offset <= 0 {
			handler.logger.Error("offset param is invalid")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
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
		handler.logger.Errorf("error getting list token transfers by contract address hash from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
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
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		return
	}

	if string(ctx.QueryArgs().Peek("blockscout")) != "true" {
		handler.logger.Error("invalid params")
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	if string(ctx.QueryArgs().Peek("page")) != "" {
		page, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("page")), 10, 0)
		if err != nil || page <= 0 {
			handler.logger.Error("page param is invalid")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
	}
	queryParams = append(queryParams, "page")
	mappingParams["page"] = strconv.FormatInt(page, 10)

	if string(ctx.QueryArgs().Peek("offset")) != "" {
		offset, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("offset")), 10, 0)
		if err != nil || offset <= 0 {
			handler.logger.Error("offset param is invalid")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
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
		handler.logger.Errorf("error getting list txs by contract address hash from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
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
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		return
	}

	if string(ctx.QueryArgs().Peek("blockscout")) != "true" {
		handler.logger.Error("invalid params")
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	if string(ctx.QueryArgs().Peek("page")) != "" {
		page, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("page")), 10, 0)
		if err != nil || page <= 0 {
			handler.logger.Error("page param is invalid")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
	}
	queryParams = append(queryParams, "page")
	mappingParams["page"] = strconv.FormatInt(page, 10)

	if string(ctx.QueryArgs().Peek("offset")) != "" {
		offset, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("offset")), 10, 0)
		if err != nil || offset <= 0 {
			handler.logger.Error("offset param is invalid")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
	}
	queryParams = append(queryParams, "offset")
	mappingParams["offset"] = strconv.FormatInt(offset, 10)
	//

	tokensAddressResp, err := handler.blockscoutClient.GetTokenHoldersOfAContractAddress(addressHash, queryParams, mappingParams)
	if err != nil {
		handler.logger.Errorf("error getting list token holders of a contract address hash from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
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
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		return
	}

	if string(ctx.QueryArgs().Peek("blockscout")) != "true" {
		handler.logger.Error("invalid params")
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	if string(ctx.QueryArgs().Peek("page")) != "" {
		page, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("page")), 10, 0)
		if err != nil || page <= 0 {
			handler.logger.Error("page param is invalid")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
	}
	queryParams = append(queryParams, "page")
	mappingParams["page"] = strconv.FormatInt(page, 10)

	if string(ctx.QueryArgs().Peek("offset")) != "" {
		offset, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("offset")), 10, 0)
		if err != nil || offset <= 0 {
			handler.logger.Error("offset param is invalid")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
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
		handler.logger.Errorf("error getting token inventory of a contract address hash from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
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
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		return
	}

	tokenId, tokenParamOk := URLValueGuard(ctx, handler.logger, "tokenid")
	if !tokenParamOk {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		return
	}

	if string(ctx.QueryArgs().Peek("blockscout")) != "true" {
		handler.logger.Error("invalid params")
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	if string(ctx.QueryArgs().Peek("page")) != "" {
		page, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("page")), 10, 0)
		if err != nil || page <= 0 {
			handler.logger.Error("page param is invalid")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
	}
	queryParams = append(queryParams, "page")
	mappingParams["page"] = strconv.FormatInt(page, 10)

	if string(ctx.QueryArgs().Peek("offset")) != "" {
		offset, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("offset")), 10, 0)
		if err != nil || offset <= 0 {
			handler.logger.Error("offset param is invalid")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
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
		handler.logger.Errorf("error getting token transfer by token id from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
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
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		return
	}
	//

	sourceCode, err := handler.blockscoutClient.GetSourceCodeByContractAddressHash(addressHash)
	if err != nil {
		handler.logger.Errorf("error getting source code of a contract address hash from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
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
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		return
	}
	//

	sourceCode, err := handler.blockscoutClient.GetTokenDetail(addressHash)
	if err != nil {
		handler.logger.Errorf("error getting token detail of a contract address hash from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, sourceCode)
}
