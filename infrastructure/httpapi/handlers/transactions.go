package handlers

import (
	"encoding/json"
	"errors"

	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/AstraProtocol/astra-indexing/external/utctime"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
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
	type Log struct {
		Address string   `json:"address"`
		Data    string   `json:"data"`
		Index   string   `json:"index"`
		Topics  []string `json:"topics"`
	}

	type TransactionEvm struct {
		BlockHeight                  int64           `json:"blockHeight"`
		BlockHash                    string          `json:"blockHash"`
		BlockTime                    utctime.UTCTime `json:"blockTime"`
		Confirmations                int64           `json:"confirmations"`
		Hash                         string          `json:"hash"`
		CosmosHash                   string          `json:"cosmosHash"`
		Index                        int             `json:"index"`
		Success                      bool            `json:"success"`
		Error                        string          `json:"error"`
		RevertReason                 string          `json:"revertReason"`
		CreatedContractCodeIndexedAt utctime.UTCTime `json:"createdContractCodeIndexedAt"`
		From                         string          `json:"from"`
		To                           string          `json:"to"`
		Value                        string          `json:"value"`
		CumulativeGasUsed            string          `json:"cumulativeGasUsed"`
		GasLimit                     string          `json:"gasLimit"`
		GasPrice                     string          `json:"gasPrice"`
		GasUsed                      string          `json:"gasUsed"`
		MaxFeePerGas                 string          `json:"maxFeePerGas"`
		MaxPriorityFeePerGas         string          `json:"maxPriorityFeePerGas"`
		Input                        string          `json:"input"`
		Nonce                        int             `json:"nonce"`
		R                            string          `json:"r"`
		S                            string          `json:"s"`
		V                            string          `json:"v"`
		Type                         int             `json:"type"`
		Logs                         []Log           `json:"logs"`
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

	if transaction.Messages[0].Type == event.MSG_ETHEREUM_TX {
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
			httpapi.Success(ctx, transaction)
			return
		}
		httpapi.Success(ctx, result.Result)
	} else {
		httpapi.Success(ctx, transaction)
	}
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
