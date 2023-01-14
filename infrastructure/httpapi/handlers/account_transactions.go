package handlers

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

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
	accountGasUsedTotalView      *account_transaction_view.AccountGasUsedTotal
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
		account_transaction_view.NewAccountGasUsedTotal(rdbHandle),
	}
}

func (handler *AccountTransactions) GetCounters(ctx *fasthttp.RequestCtx) {
	accountParam, accountParamOk := URLValueGuard(ctx, handler.logger, "account")
	if !accountParamOk {
		return
	}

	addressCounterRespChan := make(chan blockscout_infrastructure.AddressCounterResp)
	var blockscoutSearchParam string
	var astraAddress string

	// Using simultaneously blockscout get address counters api
	if evm_utils.IsHexAddress(accountParam) {
		blockscoutSearchParam = accountParam
		converted, _ := hex.DecodeString(accountParam[2:])
		astraAddress, _ = tmcosmosutils.EncodeHexToAddress("astra", converted)
	} else {
		if tmcosmosutils.IsValidCosmosAddress(accountParam) {
			astraAddress = accountParam
			_, converted, _ := tmcosmosutils.DecodeAddressToHex(accountParam)
			blockscoutSearchParam = "0x" + hex.EncodeToString(converted)
		}
	}
	go handler.blockscoutClient.GetAddressCountersAsync(blockscoutSearchParam, addressCounterRespChan)

	numberOfTxs, err := handler.accountTransactionsTotalView.Total.FindBy(fmt.Sprintf("%s:-", astraAddress))

	blockscoutAddressCounterResp := <-addressCounterRespChan
	addressCounter := blockscoutAddressCounterResp.Result

	if err == nil && addressCounter.Type != "contractaddress" {
		addressCounter.TransactionCount = numberOfTxs
	}

	totalGasUsed, err := handler.accountGasUsedTotalView.Total.FindBy(strings.ToLower(blockscoutSearchParam))
	if err == nil && addressCounter.Type != "contractaddress" {
		addressCounter.GasUsageCount = totalGasUsed
	}

	if addressCounter.Type == "" {
		addressCounter.Type = "address"
	}

	httpapi.Success(ctx, addressCounter)
}

func (handler *AccountTransactions) GetTopAddressesBalance(ctx *fasthttp.RequestCtx) {
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
		httpapi.InternalServerError(ctx)
		return
	}

	if string(ctx.QueryArgs().Peek("page")) != "" {
		page, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("page")), 10, 0)
		if err != nil || page <= 0 {
			handler.logger.Error("page param is invalid")
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
			httpapi.InternalServerError(ctx)
			return
		}
	}
	queryParams = append(queryParams, "offset")
	mappingParams["offset"] = strconv.FormatInt(offset, 10)

	if string(ctx.QueryArgs().Peek("fetched_coin_balance")) != "" {
		queryParams = append(queryParams, "fetched_coin_balance")
		mappingParams["fetched_coin_balance"] = string(ctx.QueryArgs().Peek("fetched_coin_balance"))
	}

	if string(ctx.QueryArgs().Peek("hash")) != "" {
		queryParams = append(queryParams, "hash")
		mappingParams["hash"] = string(ctx.QueryArgs().Peek("hash"))
	}
	//

	topAddressesBalanceResp, err := handler.blockscoutClient.GetTopAddressesBalance(queryParams, mappingParams)
	if err != nil {
		handler.logger.Errorf("error getting top addresses balance from blockscout: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}

	identities := make([]string, 0)
	for _, topAddressesBalanceResult := range topAddressesBalanceResp.Result {
		if evm_utils.IsHexAddress(topAddressesBalanceResult.Address) {
			converted, _ := hex.DecodeString(topAddressesBalanceResult.Address[2:])
			astraAddress, _ := tmcosmosutils.EncodeHexToAddress("astra", converted)
			identities = append(identities, astraAddress+":-")
		}
	}

	mappingAddressTotal, err := handler.accountTransactionsTotalView.Total.FindByList(identities)
	if err != nil {
		handler.logger.Errorf("error getting list total: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}

	for index, topAddressesBalanceResult := range topAddressesBalanceResp.Result {
		if evm_utils.IsHexAddress(topAddressesBalanceResult.Address) {
			converted, _ := hex.DecodeString(topAddressesBalanceResult.Address[2:])
			astraAddress, _ := tmcosmosutils.EncodeHexToAddress("astra", converted)
			topAddressesBalanceResp.Result[index].TxnCount = mappingAddressTotal[astraAddress+":-"]
		}
	}

	httpapi.Success(ctx, topAddressesBalanceResp)
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

	if evm_utils.IsHexAddress(account) {
		converted, _ := hex.DecodeString(account[2:])
		account, _ = tmcosmosutils.EncodeHexToAddress("astra", converted)
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
