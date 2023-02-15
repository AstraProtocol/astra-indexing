package handlers

import (
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/AstraProtocol/astra-indexing/appinterface/cosmosapp"
	status_polling "github.com/AstraProtocol/astra-indexing/appinterface/polling"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/AstraProtocol/astra-indexing/external/primptr"
	"github.com/AstraProtocol/astra-indexing/external/tmcosmosutils"
	evm_utils "github.com/AstraProtocol/astra-indexing/internal/evm"
	validator_view "github.com/AstraProtocol/astra-indexing/projection/validator/view"
	"github.com/AstraProtocol/astra-indexing/usecase/coin"

	"github.com/valyala/fasthttp"

	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	blockscout_infrastructure "github.com/AstraProtocol/astra-indexing/infrastructure/blockscout"
	"github.com/AstraProtocol/astra-indexing/infrastructure/httpapi"
	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"
	account_view "github.com/AstraProtocol/astra-indexing/projection/account/view"
)

type Accounts struct {
	logger applogger.Logger

	accountsView     account_view.Accounts
	validatorsView   *validator_view.Validators
	cosmosClient     cosmosapp.Client
	blockscoutClient blockscout_infrastructure.HTTPClient
	statusView       *status_polling.Status

	validatorAddressPrefix string
}

func NewAccounts(
	logger applogger.Logger,
	rdbHandle *rdb.Handle,
	cosmosClient cosmosapp.Client,
	blockscoutClient blockscout_infrastructure.HTTPClient,
	validatorAddressPrefix string,
) *Accounts {
	return &Accounts{
		logger.WithFields(applogger.LogFields{
			"module": "AccountsHandler",
		}),

		account_view.NewAccountsView(rdbHandle),
		validator_view.NewValidators(rdbHandle),
		cosmosClient,
		blockscoutClient,
		status_polling.NewStatus(rdbHandle),

		validatorAddressPrefix,
	}
}

func (handler *Accounts) GetDetailAddress(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetDetailAddress"
	accountParam, accountParamOk := URLValueGuard(ctx, handler.logger, "account")
	if !accountParamOk {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		return
	}

	addressRespChan := make(chan blockscout_infrastructure.AddressResp)

	// Using simultaneously blockscout get address detail api
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
	go handler.blockscoutClient.GetDetailAddressByAddressHashAsync(addressHash, addressRespChan)

	info := AccountInfo{
		Balance: coin.NewEmptyCoins(),
	}

	if balance, queryErr := handler.cosmosClient.Balances(accountParam); queryErr != nil {
		handler.logger.Errorf("error fetching account balance: %v", queryErr)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	} else {
		info.Balance = balance
	}

	vestingBalances, _ := handler.cosmosClient.VestingBalances(accountParam)

	var addressDetail blockscout_infrastructure.Address

	blockscoutAddressResp := <-addressRespChan
	if blockscoutAddressResp.Status == "1" {
		addressDetail = blockscoutAddressResp.Result
		addressDetail.Balance = info.Balance.AmountOf("aastra").String()
	} else {
		rawLatestHeight, err := handler.statusView.FindBy("LatestHeight")
		if err != nil {
			handler.logger.Errorf("error fetching latest height: %v", err)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}

		var latestHeight int64 = 0
		if rawLatestHeight != "" {
			// TODO: Use big.Int
			if n, err := strconv.ParseInt(rawLatestHeight, 10, 64); err != nil {
				handler.logger.Errorf("error converting latest height from string to int64: %v", err)
				prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
				httpapi.InternalServerError(ctx)
				return
			} else {
				latestHeight = n
			}
		}

		addressDetail.Balance = info.Balance.AmountOf("aastra").String()
		addressDetail.LastBalanceUpdate = latestHeight
		addressDetail.Type = "address"
		addressDetail.Verified = false
	}
	addressDetail.VestingBalances = vestingBalances

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, addressDetail)
}

func (handler *Accounts) FindBy(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "FindByAccount"
	accountParam, accountParamOk := URLValueGuard(ctx, handler.logger, "account")
	if !accountParamOk {
		return
	}

	info := AccountInfo{
		Balance:             coin.NewEmptyCoins(),
		BondedBalance:       coin.NewEmptyCoins(),
		RedelegatingBalance: coin.NewEmptyCoins(),
		UnbondingBalance:    coin.NewEmptyCoins(),
		TotalRewards:        coin.NewEmptyDecCoins(),
		Commissions:         coin.NewEmptyDecCoins(),
		TotalBalance:        coin.NewEmptyDecCoins(),
	}
	account, err := handler.cosmosClient.Account(accountParam)
	if err != nil {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.NotFound(ctx)
		return
	}

	info.Type = account.Type
	info.Address = account.Address
	if info.Type == cosmosapp.ACCOUNT_MODULE {
		info.Name = account.MaybeModuleAccount.Name
	}

	if balance, queryErr := handler.cosmosClient.Balances(accountParam); queryErr != nil {
		handler.logger.Errorf("error fetching account balance: %v", queryErr)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	} else {
		info.Balance = balance
	}

	if bondedBalance, queryErr := handler.cosmosClient.BondedBalance(accountParam); queryErr != nil {
		if !errors.Is(queryErr, cosmosapp.ErrAccountNotFound) && !errors.Is(queryErr, cosmosapp.ErrAccountNoDelegation) {
			handler.logger.Errorf("error fetching account bonded balance: %v", queryErr)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
	} else {
		info.BondedBalance = bondedBalance
	}

	if redelegatingBalance, queryErr := handler.cosmosClient.RedelegatingBalance(accountParam); queryErr != nil {
		handler.logger.Errorf("error fetching account redelegating balance: %v", queryErr)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	} else {
		info.RedelegatingBalance = redelegatingBalance
	}

	if unbondingBalance, queryErr := handler.cosmosClient.UnbondingBalance(accountParam); queryErr != nil {
		handler.logger.Errorf("error fetching account unbonding balance: %v", queryErr)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	} else {
		info.UnbondingBalance = unbondingBalance
	}

	if totalRewards, queryErr := handler.cosmosClient.TotalRewards(accountParam); queryErr != nil {
		handler.logger.Errorf("error fetching account total rewards: %v", queryErr)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	} else {
		info.TotalRewards = totalRewards
	}

	validator, err := handler.validatorsView.FindBy(validator_view.ValidatorIdentity{
		MaybeOperatorAddress: primptr.String(tmcosmosutils.MustValidatorAddressFromAccountAddress(
			handler.validatorAddressPrefix, accountParam,
		)),
	})
	if err != nil {
		if !errors.Is(err, rdb.ErrNoRows) {
			handler.logger.Errorf("error fetching account's validator: %v", err)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
		// account does not have validator
		info.Commissions = coin.NewEmptyDecCoins()
	} else {
		// account has validator
		commissions, commissionErr := handler.cosmosClient.Commission(validator.OperatorAddress)
		if commissionErr != nil {
			handler.logger.Errorf("error fetching account commissions: %v", commissionErr)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
		info.Commissions = commissions
	}

	totalBalance := coin.NewEmptyDecCoins()
	totalBalance = totalBalance.Add(coin.NewDecCoinsFromCoins(info.Balance...)...)
	totalBalance = totalBalance.Add(coin.NewDecCoinsFromCoins(info.BondedBalance...)...)
	totalBalance = totalBalance.Add(coin.NewDecCoinsFromCoins(info.RedelegatingBalance...)...)
	totalBalance = totalBalance.Add(coin.NewDecCoinsFromCoins(info.UnbondingBalance...)...)
	totalBalance = totalBalance.Add(info.TotalRewards...)
	totalBalance = totalBalance.Add(info.Commissions...)
	info.TotalBalance = totalBalance

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, info)
}

func (handler *Accounts) List(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "ListAccounts"
	var err error

	pagination, err := httpapi.ParsePagination(ctx)
	if err != nil {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	addressOrder := view.ORDER_ASC
	queryArgs := ctx.QueryArgs()
	if queryArgs.Has("order") {
		if string(queryArgs.Peek("order")) == "address.desc" {
			addressOrder = view.ORDER_DESC
		}
	}

	accounts, paginationResult, err := handler.accountsView.List(account_view.AccountsListOrder{
		AccountAddress: addressOrder,
	}, pagination)
	if err != nil {
		handler.logger.Errorf("error listing addresses: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.SuccessWithPagination(ctx, accounts, paginationResult)
}

func (handler *Accounts) GetListTokens(ctx *fasthttp.RequestCtx) {
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

type AccountInfo struct {
	Type                string        `json:"type"`
	Name                string        `json:"name"`
	Address             string        `json:"address"`
	Balance             coin.Coins    `json:"balance"`
	BondedBalance       coin.Coins    `json:"bondedBalance"`
	RedelegatingBalance coin.Coins    `json:"redelegatingBalance"`
	UnbondingBalance    coin.Coins    `json:"unbondingBalance"`
	TotalRewards        coin.DecCoins `json:"totalRewards"`
	Commissions         coin.DecCoins `json:"commissions"`
	TotalBalance        coin.DecCoins `json:"totalBalance"`
}
