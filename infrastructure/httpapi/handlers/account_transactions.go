package handlers

import (
	"encoding/hex"
	"fmt"

	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/AstraProtocol/astra-indexing/external/tmcosmosutils"
	evm_utils "github.com/AstraProtocol/astra-indexing/internal/evm"
	"github.com/valyala/fasthttp"

	"github.com/AstraProtocol/astra-indexing/appinterface/cosmosapp"
	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	blockscout_infrastructure "github.com/AstraProtocol/astra-indexing/infrastructure/blockscout"
	"github.com/AstraProtocol/astra-indexing/infrastructure/httpapi"
	account_transaction_view "github.com/AstraProtocol/astra-indexing/projection/account_transaction/view"
)

type AccountTransactions struct {
	logger applogger.Logger

	cosmosClient                 cosmosapp.Client
	blockscoutClient             blockscout_infrastructure.HTTPClient
	accountTransactionsView      *account_transaction_view.AccountTransactions
	accountTransactionsTotalView *account_transaction_view.AccountTransactionsTotal
}

func NewAccountTransactions(
	logger applogger.Logger,
	rdbHandle *rdb.Handle,
	cosmosClient cosmosapp.Client,
	blockscoutClient blockscout_infrastructure.HTTPClient,
) *AccountTransactions {
	return &AccountTransactions{
		logger.WithFields(applogger.LogFields{
			"module": "AccountTransactionsHandler",
		}),

		cosmosClient,
		blockscoutClient,
		account_transaction_view.NewAccountTransactions(rdbHandle),
		account_transaction_view.NewAccountTransactionsTotal(rdbHandle),
	}
}

func (handler *AccountTransactions) GetCounters(ctx *fasthttp.RequestCtx) {
	accountParam, accountParamOk := URLValueGuard(ctx, handler.logger, "account")
	if !accountParamOk {
		return
	}

	addressCounterRespChan := make(chan blockscout_infrastructure.AddressCounterResp)

	// Using simultaneously blockscout get address counters api
	var addressHash string
	if evm_utils.IsHexAddress(accountParam) {
		addressHash = accountParam
		converted, _ := hex.DecodeString(accountParam[2:])
		accountParam, _ = tmcosmosutils.EncodeHexToAddress("astra", converted)
	} else {
		if tmcosmosutils.IsValidCosmosAddress(accountParam) {
			_, converted, _ := tmcosmosutils.DecodeAddressToHex(accountParam)
			addressHash = "0x" + hex.EncodeToString(converted)
		}
	}
	go handler.blockscoutClient.GetAddressCountersAsync(addressHash, addressCounterRespChan)

	_, err := handler.cosmosClient.Account(accountParam)
	if err != nil {
		httpapi.NotFound(ctx)
		return
	}

	numberOfTxs, err := handler.accountTransactionsTotalView.Total.FindBy(fmt.Sprintf("%s:-", accountParam))

	blockscoutAddressCounterResp := <-addressCounterRespChan
	addressCounter := blockscoutAddressCounterResp.Result

	if err == nil && numberOfTxs > addressCounter.TransactionCount {
		addressCounter.TransactionCount = numberOfTxs
	}

	httpapi.Success(ctx, addressCounter)
}

func (handler *AccountTransactions) ListByAccount(ctx *fasthttp.RequestCtx) {
	pagination, err := httpapi.ParsePagination(ctx)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	account, accountOk := URLValueGuard(ctx, handler.logger, "account")
	if !accountOk {
		return
	}

	queryArgs := ctx.QueryArgs()

	idOrder := view.ORDER_ASC
	if queryArgs.Has("order") {
		if string(queryArgs.Peek("order")) == "height.desc" {
			idOrder = view.ORDER_DESC
		}
	}
	memo := ""
	if queryArgs.Has("memo") {
		memo = string(queryArgs.Peek("memo"))
	}

	filter := account_transaction_view.AccountTransactionsListFilter{
		Account: account,
		Memo:    memo,
	}

	blocks, paginationResult, err := handler.accountTransactionsView.List(
		filter, account_transaction_view.AccountTransactionsListOrder{Id: idOrder}, pagination,
	)
	if err != nil {
		handler.logger.Errorf("error listing account transactions: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}

	httpapi.SuccessWithPagination(ctx, blocks, paginationResult)
}
