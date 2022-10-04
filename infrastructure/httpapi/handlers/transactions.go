package handlers

import (
	"encoding/json"
	"errors"

	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/valyala/fasthttp"

	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/infrastructure/httpapi"
	transaction_view "github.com/AstraProtocol/astra-indexing/projection/transaction/view"
)

type Transactions struct {
	logger applogger.Logger

	transactionsView transaction_view.BlockTransactions

	blockscoutUrl string
}

func NewTransactions(logger applogger.Logger, rdbHandle *rdb.Handle, blockscoutUrl string) *Transactions {
	return &Transactions{
		logger.WithFields(applogger.LogFields{
			"module": "TransactionsHandler",
		}),
		transaction_view.NewTransactionsView(rdbHandle),
		blockscoutUrl,
	}
}

func (handler *Transactions) FindByHash(ctx *fasthttp.RequestCtx) {
	hashParam, hashParamOk := URLValueGuard(ctx, handler.logger, "hash")
	if !hashParamOk {
		return
	}

	transaction, err := handler.transactionsView.FindByHash(hashParam)
	if err != nil {
		if errors.Is(err, rdb.ErrNoRows) {
			httpapi.NotFound(ctx)
			return
		}
		handler.logger.Errorf("error finding transactions by hash: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}

	httpapi.Success(ctx, transaction)
}

func (handler *Transactions) FindEvmByHash(ctx *fasthttp.RequestCtx) {
	type Log struct {
		Address string   `json:"address"`
		Data    string   `json:"data"`
		Index   string   `json:"index"`
		Topics  []string `json:"topics"`
	}

	type TransactionEvm struct {
		BlockHash                    string `json:"block_hash"`
		BlockHeight                  string `json:"block_height"`
		BlockTime                    string `json:"block_time"`
		Confirmations                string `json:"confirmations"`
		Hash                         string `json:"hash"`
		CosmosHash                   string `json:"cosmos_hash"`
		CreatedContractCodeIndexedAt string `json:"created_contract_code_indexed_at"`
		CumulativeGasUsed            string `json:"cumulative_gas_used"`
		Error                        string `json:"error"`
		RevertReason                 string `json:"revert_reason"`
		From                         string `json:"from"`
		To                           string `json:"to"`
		Value                        string `json:"value"`
		GasLimit                     string `json:"gas_limit"`
		GasPrice                     string `json:"gas_price"`
		GasUsed                      string `json:"gas_used"`
		MaxFeePerGas                 string `json:"maxFeePerGas"`
		MaxPriorityFeePerGas         string `json:"maxPriorityFeePerGas"`
		Index                        string `json:"index"`
		Input                        string `json:"input"`
		Success                      bool   `json:"success"`
		Nonce                        string `json:"nonce"`
		R                            string `json:"r"`
		S                            string `json:"s"`
		V                            string `json:"v"`
		Type                         string `json:"type"`
		Logs                         []Log  `json:"logs"`
	}

	type Result struct {
		Message string         `json:"message"`
		Result  TransactionEvm `json:"result"`
		Status  string         `json:"status"`
	}

	hashParam, hashParamOk := URLValueGuard(ctx, handler.logger, "hash")
	if !hashParamOk {
		return
	}
	base_url := handler.blockscoutUrl
	url := base_url + "/api/v1?module=transaction&action=getTxCosmosInfo&txhash=" + hashParam
	req := fasthttp.AcquireRequest()
	req.SetRequestURI(url)

	resp := fasthttp.AcquireResponse()
	client := &fasthttp.Client{}
	client.Do(req, resp)

	result := Result{}
	bodyBytes := resp.Body()

	err := json.Unmarshal(bodyBytes, &result)
	if err != nil {
		handler.logger.Errorf("error parsing response from endpoint %s: %v", url, err)
		httpapi.InternalServerError(ctx)
		return
	}
	httpapi.Success(ctx, result.Result)
}

func (handler *Transactions) List(ctx *fasthttp.RequestCtx) {
	pagination, err := httpapi.ParsePagination(ctx)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	heightOrder := view.ORDER_ASC
	queryArgs := ctx.QueryArgs()
	if queryArgs.Has("order") {
		if string(queryArgs.Peek("order")) == "height.desc" {
			heightOrder = view.ORDER_DESC
		}
	}

	blocks, paginationResult, err := handler.transactionsView.List(transaction_view.TransactionsListFilter{
		MaybeBlockHeight: nil,
	}, transaction_view.TransactionsListOrder{
		Height: heightOrder,
	}, pagination)
	if err != nil {
		handler.logger.Errorf("error listing transactions: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}

	httpapi.SuccessWithPagination(ctx, blocks, paginationResult)
}
