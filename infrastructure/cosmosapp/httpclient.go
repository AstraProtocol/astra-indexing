package cosmosapp

import (
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/AstraProtocol/astra-indexing/external/cache"
	utils "github.com/AstraProtocol/astra-indexing/infrastructure"
	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"
	"github.com/AstraProtocol/astra-indexing/usecase/coin"
	"github.com/AstraProtocol/astra-indexing/usecase/model"
	"github.com/hashicorp/go-retryablehttp"
	jsoniter "github.com/json-iterator/go"

	cosmosapp_interface "github.com/AstraProtocol/astra-indexing/appinterface/cosmosapp"
)

var _ cosmosapp_interface.Client = &HTTPClient{}

var (
	redirectsErrorRe = regexp.MustCompile(`stopped after \d+ redirects\z`)
	// specifically so we resort to matching on the error string.
	schemeErrorRe = regexp.MustCompile(`unsupported protocol scheme`)

	// A regular expression to match the error returned by net/http when the
	// TLS certificate is not trusted. This error isn't typed
	// specifically so we resort to matching on the error string.
	notTrustedErrorRe = regexp.MustCompile(`certificate is not trusted`)
)

const ERR_CODE_ACCOUNT_NOT_FOUND = 2
const ERR_CODE_ACCOUNT_NO_DELEGATION = 5

type HTTPClient struct {
	httpClient   *retryablehttp.Client
	rpcUrl       string
	bondingDenom string
	httpCache    *cache.AstraCache
}

// DefaultRetryPolicy provides a default callback for Client.CheckRetry, which
// will retry on connection errors and server errors.
func defaultRetryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	// do not retry on context.Canceled or context.DeadlineExceeded
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	// don't propagate other errors
	shouldRetry, _ := baseRetryPolicy(resp, err)
	return shouldRetry, nil
}

func baseRetryPolicy(resp *http.Response, err error) (bool, error) {
	if err != nil {
		if v, ok := err.(*url.Error); ok {
			// Don't retry if the error was due to too many redirects.
			if redirectsErrorRe.MatchString(v.Error()) {
				return false, v
			}

			// Don't retry if the error was due to an invalid protocol scheme.
			if schemeErrorRe.MatchString(v.Error()) {
				return false, v
			}

			// Don't retry if the error was due to TLS cert verification failure.
			if notTrustedErrorRe.MatchString(v.Error()) {
				return false, v
			}
			if _, ok := v.Err.(x509.UnknownAuthorityError); ok {
				return false, v
			}
		}

		// The error is likely recoverable so retry.
		return true, nil
	}

	// 429 Too Many Requests is recoverable. Sometimes the server puts
	// a Retry-After response header to indicate when the server is
	// available to start processing request from client.
	if resp.StatusCode == http.StatusTooManyRequests {
		return true, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	// Check the response code. We retry on 500-range responses to allow
	// the server time to recover, as 500's are typically not permanent
	// errors and may relate to outages on the server side. This will catch
	// invalid response codes as well, like 0 and 999.
	if resp.StatusCode == 0 || (resp.StatusCode >= 500 && resp.StatusCode != http.StatusNotImplemented) {
		return true, fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}

	return false, nil
}

// NewHTTPClient returns a new HTTPClient for tendermint request
func NewHTTPClient(rpcUrl string, bondingDenom string) *HTTPClient {
	httpClient := retryablehttp.NewClient()
	httpClient.Logger = nil
	httpClient.CheckRetry = defaultRetryPolicy

	return &HTTPClient{
		httpClient,
		strings.TrimSuffix(rpcUrl, "/"),
		bondingDenom,
		cache.NewCache(),
	}
}

func (client *HTTPClient) Account(accountAddress string) (*cosmosapp_interface.Account, error) {
	cacheKey := fmt.Sprintf("CosmosAccount_%s", accountAddress)
	var accountTmp cosmosapp_interface.Account

	err := client.httpCache.Get(cacheKey, &accountTmp)
	if err == nil {
		return &accountTmp, nil
	}

	rawRespBody, err := client.request(
		fmt.Sprintf("%s/%s", client.getUrl("auth", "accounts"), accountAddress), "",
	)
	if err != nil {
		return nil, err
	}
	defer rawRespBody.Close()

	var accountResp AccountResp
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&accountResp); err != nil {
		return nil, err
	}
	rawAccount := accountResp.Account

	var account cosmosapp_interface.Account
	switch rawAccount.Type {
	case cosmosapp_interface.ACCOUNT_BASE:
		account = cosmosapp_interface.Account{
			Type:          rawAccount.Type,
			Address:       *rawAccount.MaybeAddress,
			MaybePubkey:   rawAccount.MaybePubKey,
			AccountNumber: *rawAccount.MaybeAccountNumber,
			Sequence:      *rawAccount.MaybeSequence,

			MaybeModuleAccount:            nil,
			MaybeDelayedVestingAccount:    nil,
			MaybeContinuousVestingAccount: nil,
			MaybePeriodicVestingAccount:   nil,
		}
	case cosmosapp_interface.ACCOUNT_ETHERMINT:
		account = cosmosapp_interface.Account{
			Type:          rawAccount.Type,
			Address:       rawAccount.MaybeBaseAccount.Address,
			MaybePubkey:   rawAccount.MaybeBaseAccount.MaybePubKey,
			AccountNumber: rawAccount.MaybeBaseAccount.AccountNumber,
			Sequence:      rawAccount.MaybeBaseAccount.Sequence,

			MaybeDelayedVestingAccount:    nil,
			MaybeContinuousVestingAccount: nil,
			MaybePeriodicVestingAccount:   nil,
		}
	case cosmosapp_interface.ACCOUNT_MODULE:
		account = cosmosapp_interface.Account{
			Type:          rawAccount.Type,
			Address:       rawAccount.MaybeBaseAccount.Address,
			MaybePubkey:   nil,
			AccountNumber: rawAccount.MaybeBaseAccount.AccountNumber,
			Sequence:      rawAccount.MaybeBaseAccount.Sequence,
			MaybeModuleAccount: &cosmosapp_interface.ModuleAccount{
				Name:        *rawAccount.MaybeName,
				Permissions: rawAccount.MaybePermissions,
			},

			MaybeDelayedVestingAccount:    nil,
			MaybeContinuousVestingAccount: nil,
			MaybePeriodicVestingAccount:   nil,
		}
	case "/cosmos.vesting.v1beta1.DelayedVestingAccount":
		account = cosmosapp_interface.Account{
			Type:          rawAccount.Type,
			Address:       rawAccount.MaybeBaseVestingAccount.BaseAccount.Address,
			MaybePubkey:   rawAccount.MaybeBaseVestingAccount.BaseAccount.MaybePubKey,
			AccountNumber: rawAccount.MaybeBaseVestingAccount.BaseAccount.AccountNumber,
			Sequence:      rawAccount.MaybeBaseVestingAccount.BaseAccount.Sequence,

			MaybeModuleAccount: nil,
			MaybeDelayedVestingAccount: &cosmosapp_interface.DelayedVestingAccount{
				OriginalVesting:  rawAccount.MaybeBaseVestingAccount.OriginalVesting,
				DelegatedFree:    rawAccount.MaybeBaseVestingAccount.DelegatedFree,
				DelegatedVesting: rawAccount.MaybeBaseVestingAccount.DelegatedVesting,
				EndTime:          rawAccount.MaybeBaseVestingAccount.EndTime,
			},
			MaybeContinuousVestingAccount: nil,
			MaybePeriodicVestingAccount:   nil,
		}

	case "/cosmos.vesting.v1beta1.ContinuousVestingAccount":
		account = cosmosapp_interface.Account{
			Type:          rawAccount.Type,
			Address:       rawAccount.MaybeBaseVestingAccount.BaseAccount.Address,
			MaybePubkey:   rawAccount.MaybeBaseVestingAccount.BaseAccount.MaybePubKey,
			AccountNumber: rawAccount.MaybeBaseVestingAccount.BaseAccount.AccountNumber,
			Sequence:      rawAccount.MaybeBaseVestingAccount.BaseAccount.Sequence,

			MaybeModuleAccount:         nil,
			MaybeDelayedVestingAccount: nil,
			MaybeContinuousVestingAccount: &cosmosapp_interface.ContinuousVestingAccount{
				OriginalVesting:  rawAccount.MaybeBaseVestingAccount.OriginalVesting,
				DelegatedFree:    rawAccount.MaybeBaseVestingAccount.DelegatedFree,
				DelegatedVesting: rawAccount.MaybeBaseVestingAccount.DelegatedVesting,
				StartTime:        *rawAccount.MaybeStartTime,
				EndTime:          rawAccount.MaybeBaseVestingAccount.EndTime,
			},
			MaybePeriodicVestingAccount: nil,
		}
	case cosmosapp_interface.ACCOUNT_VESTING_PERIODIC:
		account = cosmosapp_interface.Account{
			Type:          rawAccount.Type,
			Address:       rawAccount.MaybeBaseVestingAccount.BaseAccount.Address,
			MaybePubkey:   rawAccount.MaybeBaseVestingAccount.BaseAccount.MaybePubKey,
			AccountNumber: rawAccount.MaybeBaseVestingAccount.BaseAccount.AccountNumber,
			Sequence:      rawAccount.MaybeBaseVestingAccount.BaseAccount.Sequence,

			MaybeModuleAccount:            nil,
			MaybeDelayedVestingAccount:    nil,
			MaybeContinuousVestingAccount: nil,
			MaybePeriodicVestingAccount: &cosmosapp_interface.PeriodicVestingAccount{
				OriginalVesting:  rawAccount.MaybeBaseVestingAccount.OriginalVesting,
				DelegatedFree:    rawAccount.MaybeBaseVestingAccount.DelegatedFree,
				DelegatedVesting: rawAccount.MaybeBaseVestingAccount.DelegatedVesting,
				StartTime:        *rawAccount.MaybeStartTime,
				EndTime:          rawAccount.MaybeBaseVestingAccount.EndTime,
				VestingPeriods:   rawAccount.MaybeVestingPeriods,
			},
		}
	case cosmosapp_interface.ACCOUNT_CLAWBACK_VESTING:
		account = cosmosapp_interface.Account{
			Type:                          rawAccount.Type,
			Address:                       rawAccount.MaybeBaseVestingAccount.BaseAccount.Address,
			MaybePubkey:                   rawAccount.MaybeBaseVestingAccount.BaseAccount.MaybePubKey,
			AccountNumber:                 rawAccount.MaybeBaseVestingAccount.BaseAccount.AccountNumber,
			Sequence:                      rawAccount.MaybeBaseVestingAccount.BaseAccount.Sequence,
			MaybeModuleAccount:            nil,
			MaybeDelayedVestingAccount:    nil,
			MaybeContinuousVestingAccount: nil,
			MaybePeriodicVestingAccount:   nil,
			MaybeClawbackVestingAccount: &cosmosapp_interface.ClawbackVestingAccount{
				FunderAddress:  *rawAccount.MaybeFunderAddress,
				StartTime:      *rawAccount.MaybeStartTime,
				LockupPeriod:   rawAccount.MaybeLockupPeriods,
				VestingPeriods: rawAccount.MaybeVestingPeriods,
			},
		}

	default:
		return nil, fmt.Errorf("unrecognized account type: %s", rawAccount.Type)
	}

	client.httpCache.Set(cacheKey, &account, utils.TIME_CACHE_FAST)

	return &account, nil
}

func (client *HTTPClient) Balances(accountAddress string) (coin.Coins, error) {
	cacheKey := fmt.Sprintf("CosmosBalances_%s", accountAddress)
	var coinsTmp coin.Coins

	err := client.httpCache.Get(cacheKey, &coinsTmp)
	if err == nil {
		return coinsTmp, nil
	}

	resp := &BankBalancesResp{
		Pagination: Pagination{
			MaybeNextKey: nil,
			Total:        "",
		},
	}
	balances := coin.NewEmptyCoins()
	for {
		queryUrl := fmt.Sprintf("%s/%s", client.getUrl("bank", "balances"), accountAddress)
		if resp.Pagination.MaybeNextKey != nil {
			queryUrl = fmt.Sprintf(
				"%s?pagination.key=%s",
				queryUrl, url.QueryEscape(*resp.Pagination.MaybeNextKey),
			)
		}

		rawRespBody, err := client.request(queryUrl)
		if err != nil {
			return nil, err
		}
		defer rawRespBody.Close()

		if err := jsoniter.NewDecoder(rawRespBody).Decode(&resp); err != nil {
			return nil, err
		}
		for _, balanceKVPair := range resp.BankBalanceResponses {
			balance, coinErr := coin.NewCoinFromString(balanceKVPair.Denom, balanceKVPair.Amount)
			if coinErr != nil {
				return nil, coinErr
			}
			balances = balances.Add(balance)
		}

		if resp.Pagination.MaybeNextKey == nil {
			break
		}
	}

	client.httpCache.Set(cacheKey, balances, utils.TIME_CACHE_FAST)

	return balances, nil
}

func (client *HTTPClient) BalancesAsync(accountAddress string, balancesChan chan coin.Coins) {
	cacheKey := fmt.Sprintf("CosmosBalancesAsync_%s", accountAddress)
	var coinsTmp coin.Coins

	err := client.httpCache.Get(cacheKey, &coinsTmp)
	if err == nil {
		balancesChan <- coinsTmp
		return
	}

	// Make sure we close these channels when we're done with them
	defer func() {
		close(balancesChan)
	}()

	resp := &BankBalancesResp{
		Pagination: Pagination{
			MaybeNextKey: nil,
			Total:        "",
		},
	}
	balances := coin.NewEmptyCoins()
	for {
		queryUrl := fmt.Sprintf("%s/%s", client.getUrl("bank", "balances"), accountAddress)
		if resp.Pagination.MaybeNextKey != nil {
			queryUrl = fmt.Sprintf(
				"%s?pagination.key=%s",
				queryUrl, url.QueryEscape(*resp.Pagination.MaybeNextKey),
			)
		}

		rawRespBody, err := client.request(queryUrl)
		if err != nil {
			balancesChan <- balances
			return
		}
		defer rawRespBody.Close()

		if err := jsoniter.NewDecoder(rawRespBody).Decode(&resp); err != nil {
			balancesChan <- balances
			return
		}
		for _, balanceKVPair := range resp.BankBalanceResponses {
			balance, coinErr := coin.NewCoinFromString(balanceKVPair.Denom, balanceKVPair.Amount)
			if coinErr != nil {
				balancesChan <- balances
				return
			}
			balances = balances.Add(balance)
		}

		if resp.Pagination.MaybeNextKey == nil {
			break
		}
	}

	client.httpCache.Set(cacheKey, balances, utils.TIME_CACHE_FAST)
	balancesChan <- balances
}

func (client *HTTPClient) BondedBalance(accountAddress string) (coin.Coins, error) {
	cacheKey := fmt.Sprintf("CosmosBondedBalance_%s", accountAddress)
	var coinsTmp coin.Coins

	err := client.httpCache.Get(cacheKey, &coinsTmp)
	if err == nil {
		return coinsTmp, nil
	}

	resp := &DelegationsResp{
		MaybePagination: &Pagination{
			MaybeNextKey: nil,
			Total:        "",
		},
	}
	balance := coin.NewEmptyCoins()
	for {
		queryUrl := fmt.Sprintf("%s/%s", client.getUrl("staking", "delegations"), accountAddress)
		if resp.MaybePagination.MaybeNextKey != nil {
			queryUrl = fmt.Sprintf(
				"%s?pagination.key=%s",
				queryUrl, url.QueryEscape(*resp.MaybePagination.MaybeNextKey),
			)
		}

		rawRespBody, statusCode, err := client.rawRequest(queryUrl)
		if err != nil {
			return nil, err
		}
		defer rawRespBody.Close()

		if decodeErr := jsoniter.NewDecoder(rawRespBody).Decode(&resp); decodeErr != nil {
			return nil, decodeErr
		}
		if resp.MaybeCode != nil {
			if *resp.MaybeCode == ERR_CODE_ACCOUNT_NOT_FOUND {
				return nil, cosmosapp_interface.ErrAccountNotFound
			} else if *resp.MaybeCode == ERR_CODE_ACCOUNT_NO_DELEGATION {
				return nil, cosmosapp_interface.ErrAccountNoDelegation
			}
		}
		if statusCode != 200 {
			return nil, fmt.Errorf("error requesting Cosmos %s endpoint: status code %d", queryUrl, statusCode)
		}
		for _, delegation := range resp.MaybeDelegationResponses {
			delegatedCoin, coinErr := coin.NewCoinFromString(delegation.Balance.Denom, delegation.Balance.Amount)
			if coinErr != nil {
				return nil, fmt.Errorf("error parsing Coin from delegation balance: %v", coinErr)
			}
			balance = balance.Add(delegatedCoin)
		}

		if resp.MaybePagination.MaybeNextKey == nil {
			break
		}
	}

	client.httpCache.Set(cacheKey, balance, utils.TIME_CACHE_FAST)

	return balance, nil
}

func (client *HTTPClient) BondedBalanceAsync(accountAddress string, bondedBalancesChan chan coin.Coins) {
	cacheKey := fmt.Sprintf("CosmosBondedBalanceAsync_%s", accountAddress)
	var coinsTmp coin.Coins

	err := client.httpCache.Get(cacheKey, &coinsTmp)
	if err == nil {
		bondedBalancesChan <- coinsTmp
		return
	}

	// Make sure we close these channels when we're done with them
	defer func() {
		close(bondedBalancesChan)
	}()

	resp := &DelegationsResp{
		MaybePagination: &Pagination{
			MaybeNextKey: nil,
			Total:        "",
		},
	}
	balance := coin.NewEmptyCoins()
	for {
		queryUrl := fmt.Sprintf("%s/%s", client.getUrl("staking", "delegations"), accountAddress)
		if resp.MaybePagination.MaybeNextKey != nil {
			queryUrl = fmt.Sprintf(
				"%s?pagination.key=%s",
				queryUrl, url.QueryEscape(*resp.MaybePagination.MaybeNextKey),
			)
		}

		rawRespBody, statusCode, err := client.rawRequest(queryUrl)
		if err != nil {
			bondedBalancesChan <- balance
			return
		}
		defer rawRespBody.Close()

		if decodeErr := jsoniter.NewDecoder(rawRespBody).Decode(&resp); decodeErr != nil {
			bondedBalancesChan <- balance
			return
		}
		if resp.MaybeCode != nil {
			if *resp.MaybeCode == ERR_CODE_ACCOUNT_NOT_FOUND {
				bondedBalancesChan <- balance
				return
			} else if *resp.MaybeCode == ERR_CODE_ACCOUNT_NO_DELEGATION {
				bondedBalancesChan <- balance
				return
			}
		}
		if statusCode != 200 {
			bondedBalancesChan <- balance
			return
		}
		for _, delegation := range resp.MaybeDelegationResponses {
			delegatedCoin, coinErr := coin.NewCoinFromString(delegation.Balance.Denom, delegation.Balance.Amount)
			if coinErr != nil {
				bondedBalancesChan <- balance
				return
			}
			balance = balance.Add(delegatedCoin)
		}

		if resp.MaybePagination.MaybeNextKey == nil {
			break
		}
	}

	client.httpCache.Set(cacheKey, balance, utils.TIME_CACHE_FAST)
	bondedBalancesChan <- balance
}

func (client *HTTPClient) RedelegatingBalance(accountAddress string) (coin.Coins, error) {
	cacheKey := fmt.Sprintf("CosmosRedelegatingBalance_%s", accountAddress)
	var coinsTmp coin.Coins

	err := client.httpCache.Get(cacheKey, &coinsTmp)
	if err == nil {
		return coinsTmp, nil
	}

	resp := &UnbondingResp{
		Pagination: Pagination{
			MaybeNextKey: nil,
			Total:        "",
		},
	}
	balance := coin.NewEmptyCoins()
	for {
		queryUrl := fmt.Sprintf(
			"%s/%s/redelegations", client.getUrl("staking", "delegators"), accountAddress,
		)
		if resp.Pagination.MaybeNextKey != nil {
			queryUrl = fmt.Sprintf(
				"%s?pagination.key=%s",
				queryUrl, url.QueryEscape(*resp.Pagination.MaybeNextKey),
			)
		}

		rawRespBody, err := client.request(queryUrl)
		if err != nil {
			return nil, err
		}
		defer rawRespBody.Close()

		if err := jsoniter.NewDecoder(rawRespBody).Decode(&resp); err != nil {
			return nil, err
		}
		for _, unbonding := range resp.UnbondingResponses {
			for _, entry := range unbonding.Entries {
				unbondingCoin, coinErr := coin.NewCoinFromString(client.bondingDenom, entry.Balance)
				if coinErr != nil {
					return nil, fmt.Errorf("error parsing Coin from unbonding balance: %v", coinErr)
				}
				balance = balance.Add(unbondingCoin)
			}
		}

		if resp.Pagination.MaybeNextKey == nil {
			break
		}
	}

	client.httpCache.Set(cacheKey, balance, utils.TIME_CACHE_FAST)

	return balance, nil
}

func (client *HTTPClient) RedelegatingBalanceAsync(accountAddress string, redelegatingBalancesChan chan coin.Coins) {
	cacheKey := fmt.Sprintf("CosmosRedelegatingBalanceAsync_%s", accountAddress)
	var coinsTmp coin.Coins

	err := client.httpCache.Get(cacheKey, &coinsTmp)
	if err == nil {
		redelegatingBalancesChan <- coinsTmp
		return
	}

	// Make sure we close these channels when we're done with them
	defer func() {
		close(redelegatingBalancesChan)
	}()

	resp := &UnbondingResp{
		Pagination: Pagination{
			MaybeNextKey: nil,
			Total:        "",
		},
	}
	balance := coin.NewEmptyCoins()
	for {
		queryUrl := fmt.Sprintf(
			"%s/%s/redelegations", client.getUrl("staking", "delegators"), accountAddress,
		)
		if resp.Pagination.MaybeNextKey != nil {
			queryUrl = fmt.Sprintf(
				"%s?pagination.key=%s",
				queryUrl, url.QueryEscape(*resp.Pagination.MaybeNextKey),
			)
		}

		rawRespBody, err := client.request(queryUrl)
		if err != nil {
			redelegatingBalancesChan <- balance
			return
		}
		defer rawRespBody.Close()

		if err := jsoniter.NewDecoder(rawRespBody).Decode(&resp); err != nil {
			redelegatingBalancesChan <- balance
			return
		}
		for _, unbonding := range resp.UnbondingResponses {
			for _, entry := range unbonding.Entries {
				unbondingCoin, coinErr := coin.NewCoinFromString(client.bondingDenom, entry.Balance)
				if coinErr != nil {
					redelegatingBalancesChan <- balance
					return
				}
				balance = balance.Add(unbondingCoin)
			}
		}

		if resp.Pagination.MaybeNextKey == nil {
			break
		}
	}

	client.httpCache.Set(cacheKey, balance, utils.TIME_CACHE_FAST)
	redelegatingBalancesChan <- balance
}

func (client *HTTPClient) UnbondingBalance(accountAddress string) (coin.Coins, error) {
	cacheKey := fmt.Sprintf("CosmosUnbondingBalance_%s", accountAddress)
	var coinsTmp coin.Coins

	err := client.httpCache.Get(cacheKey, &coinsTmp)
	if err == nil {
		return coinsTmp, nil
	}

	resp := &UnbondingResp{
		Pagination: Pagination{
			MaybeNextKey: nil,
			Total:        "",
		},
	}
	balance := coin.NewEmptyCoins()
	for {
		queryUrl := fmt.Sprintf(
			"%s/%s/unbonding_delegations",
			client.getUrl("staking", "delegators"), accountAddress,
		)
		if resp.Pagination.MaybeNextKey != nil {
			queryUrl = fmt.Sprintf(
				"%s?pagination.key=%s",
				queryUrl, url.QueryEscape(*resp.Pagination.MaybeNextKey),
			)
		}

		rawRespBody, err := client.request(queryUrl)
		if err != nil {
			return nil, err
		}
		defer rawRespBody.Close()

		if err := jsoniter.NewDecoder(rawRespBody).Decode(&resp); err != nil {
			return nil, err
		}
		for _, unbonding := range resp.UnbondingResponses {
			for _, entry := range unbonding.Entries {
				unbondingCoin, coinErr := coin.NewCoinFromString(client.bondingDenom, entry.Balance)
				if coinErr != nil {
					return nil, fmt.Errorf("error parsing Coin from unbonding balance: %v", coinErr)
				}
				balance = balance.Add(unbondingCoin)
			}
		}

		if resp.Pagination.MaybeNextKey == nil {
			break
		}
	}

	client.httpCache.Set(cacheKey, balance, utils.TIME_CACHE_FAST)

	return balance, nil
}

func (client *HTTPClient) UnbondingBalanceAsync(accountAddress string, unbondingBalancesChan chan coin.Coins) {
	cacheKey := fmt.Sprintf("CosmosUnbondingBalanceAsync_%s", accountAddress)
	var coinsTmp coin.Coins

	err := client.httpCache.Get(cacheKey, &coinsTmp)
	if err == nil {
		unbondingBalancesChan <- coinsTmp
		return
	}

	// Make sure we close these channels when we're done with them
	defer func() {
		close(unbondingBalancesChan)
	}()

	resp := &UnbondingResp{
		Pagination: Pagination{
			MaybeNextKey: nil,
			Total:        "",
		},
	}
	balance := coin.NewEmptyCoins()
	for {
		queryUrl := fmt.Sprintf(
			"%s/%s/unbonding_delegations",
			client.getUrl("staking", "delegators"), accountAddress,
		)
		if resp.Pagination.MaybeNextKey != nil {
			queryUrl = fmt.Sprintf(
				"%s?pagination.key=%s",
				queryUrl, url.QueryEscape(*resp.Pagination.MaybeNextKey),
			)
		}

		rawRespBody, err := client.request(queryUrl)
		if err != nil {
			unbondingBalancesChan <- balance
			return
		}
		defer rawRespBody.Close()

		if err := jsoniter.NewDecoder(rawRespBody).Decode(&resp); err != nil {
			unbondingBalancesChan <- balance
			return
		}
		for _, unbonding := range resp.UnbondingResponses {
			for _, entry := range unbonding.Entries {
				unbondingCoin, coinErr := coin.NewCoinFromString(client.bondingDenom, entry.Balance)
				if coinErr != nil {
					unbondingBalancesChan <- balance
					return
				}
				balance = balance.Add(unbondingCoin)
			}
		}

		if resp.Pagination.MaybeNextKey == nil {
			break
		}
	}

	client.httpCache.Set(cacheKey, balance, utils.TIME_CACHE_FAST)
	unbondingBalancesChan <- balance
}

func (client *HTTPClient) TotalRewards(accountAddress string) (coin.DecCoins, error) {
	cacheKey := fmt.Sprintf("CosmosTotalRewards_%s", accountAddress)
	var decCoinsTmp coin.DecCoins

	err := client.httpCache.Get(cacheKey, &decCoinsTmp)
	if err == nil {
		return decCoinsTmp, nil
	}

	rawRespBody, err := client.request(
		fmt.Sprintf(
			"%s/%s/rewards",
			client.getUrl("distribution", "delegators"),
			accountAddress,
		), "",
	)
	if err != nil {
		return nil, err
	}
	defer rawRespBody.Close()

	var delegatorRewardResp DelegatorRewardResp
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&delegatorRewardResp); err != nil {
		return nil, err
	}

	rewards := coin.NewEmptyDecCoins()
	for _, total := range delegatorRewardResp.Total {
		rewardCoin, coinErr := coin.NewDecCoinFromString(total.Denom, total.Amount)
		if coinErr != nil {
			return nil, fmt.Errorf("error parsing Coin from total reward: %v", coinErr)
		}
		rewards = rewards.Add(rewardCoin)
	}

	client.httpCache.Set(cacheKey, rewards, utils.TIME_CACHE_FAST)

	return rewards, nil
}

func (client *HTTPClient) TotalRewardsAsync(accountAddress string, rewardBalanceChan chan coin.DecCoins) {
	cacheKey := fmt.Sprintf("CosmosTotalRewardsAsync_%s", accountAddress)
	var decCoinsTmp coin.DecCoins

	err := client.httpCache.Get(cacheKey, &decCoinsTmp)
	if err == nil {
		rewardBalanceChan <- decCoinsTmp
		return
	}

	// Make sure we close these channels when we're done with them
	defer func() {
		close(rewardBalanceChan)
	}()

	rewards := coin.NewEmptyDecCoins()
	rawRespBody, err := client.request(
		fmt.Sprintf(
			"%s/%s/rewards",
			client.getUrl("distribution", "delegators"),
			accountAddress,
		), "",
	)
	if err != nil {
		rewardBalanceChan <- rewards
		return
	}
	defer rawRespBody.Close()

	var delegatorRewardResp DelegatorRewardResp
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&delegatorRewardResp); err != nil {
		rewardBalanceChan <- rewards
		return
	}

	for _, total := range delegatorRewardResp.Total {
		rewardCoin, coinErr := coin.NewDecCoinFromString(total.Denom, total.Amount)
		if coinErr != nil {
			rewardBalanceChan <- rewards
			return
		}
		rewards = rewards.Add(rewardCoin)
	}

	client.httpCache.Set(cacheKey, rewards, utils.TIME_CACHE_FAST)
	rewardBalanceChan <- rewards
}

func (client *HTTPClient) Validator(validatorAddress string) (*cosmosapp_interface.Validator, error) {
	cacheKey := fmt.Sprintf("CosmosValidator_%s", validatorAddress)
	var validatorTmp *cosmosapp_interface.Validator

	err := client.httpCache.Get(cacheKey, &validatorTmp)
	if err == nil {
		return validatorTmp, nil
	}

	rawRespBody, err := client.request(
		fmt.Sprintf(
			"%s/%s",
			client.getUrl("staking", "validators"), validatorAddress,
		), "",
	)
	if err != nil {
		return nil, err
	}
	defer rawRespBody.Close()

	var validatorResp ValidatorResp
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&validatorResp); err != nil {
		return nil, err
	}

	client.httpCache.Set(cacheKey, &validatorResp.Validator, utils.TIME_CACHE_FAST)

	return &validatorResp.Validator, nil
}

func (client *HTTPClient) Commission(validatorAddress string) (coin.DecCoins, error) {
	cacheKey := fmt.Sprintf("CosmosCommission_%s", validatorAddress)
	var decCoinsTmp coin.DecCoins

	err := client.httpCache.Get(cacheKey, &decCoinsTmp)
	if err == nil {
		return decCoinsTmp, nil
	}

	rawRespBody, err := client.request(
		fmt.Sprintf("%s/%s/commission",
			client.getUrl("distribution", "validators"), validatorAddress,
		), "",
	)
	if err != nil {
		return nil, err
	}
	defer rawRespBody.Close()

	var commissionResp ValidatorCommissionResp
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&commissionResp); err != nil {
		return nil, err
	}

	totalCommission := coin.NewEmptyDecCoins()
	for _, commission := range commissionResp.Commissions.Commission {
		commissionCoin, coinErr := coin.NewDecCoinFromString(commission.Denom, commission.Amount)
		if coinErr != nil {
			return nil, fmt.Errorf("error parsing Coin from commission: %v", coinErr)
		}
		totalCommission = totalCommission.Add(commissionCoin)
	}

	client.httpCache.Set(cacheKey, totalCommission, utils.TIME_CACHE_FAST)

	return totalCommission, nil
}

func (client *HTTPClient) CommissionAsync(validatorAddress string, commissionBalanceChan chan coin.DecCoins) {
	cacheKey := fmt.Sprintf("CosmosCommissionAsync_%s", validatorAddress)
	var decCoinsTmp coin.DecCoins

	err := client.httpCache.Get(cacheKey, &decCoinsTmp)
	if err == nil {
		commissionBalanceChan <- decCoinsTmp
		return
	}

	// Make sure we close these channels when we're done with them
	defer func() {
		close(commissionBalanceChan)
	}()

	totalCommission := coin.NewEmptyDecCoins()
	rawRespBody, err := client.request(
		fmt.Sprintf("%s/%s/commission",
			client.getUrl("distribution", "validators"), validatorAddress,
		), "",
	)
	if err != nil {
		commissionBalanceChan <- totalCommission
		return
	}
	defer rawRespBody.Close()

	var commissionResp ValidatorCommissionResp
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&commissionResp); err != nil {
		commissionBalanceChan <- totalCommission
		return
	}

	for _, commission := range commissionResp.Commissions.Commission {
		commissionCoin, coinErr := coin.NewDecCoinFromString(commission.Denom, commission.Amount)
		if coinErr != nil {
			commissionBalanceChan <- totalCommission
			return
		}
		totalCommission = totalCommission.Add(commissionCoin)
	}

	client.httpCache.Set(cacheKey, totalCommission, utils.TIME_CACHE_FAST)
	commissionBalanceChan <- totalCommission
}

func (client *HTTPClient) Delegation(
	delegator string, validator string,
) (*cosmosapp_interface.DelegationResponse, error) {
	cacheKey := fmt.Sprintf("CosmosDelegation_%s_%s", delegator, validator)
	var delegationResponseTmp *cosmosapp_interface.DelegationResponse

	err := client.httpCache.Get(cacheKey, &delegationResponseTmp)
	if err == nil {
		return delegationResponseTmp, nil
	}

	resp := &DelegationsResp{
		MaybePagination: &Pagination{
			MaybeNextKey: nil,
			Total:        "",
		},
	}
	for {
		queryUrl := fmt.Sprintf("%s/%s", client.getUrl("staking", "delegations"), delegator)
		if resp.MaybePagination.MaybeNextKey != nil {
			queryUrl = fmt.Sprintf(
				"%s?pagination.key=%s",
				queryUrl, url.QueryEscape(*resp.MaybePagination.MaybeNextKey),
			)
		}

		rawRespBody, statusCode, err := client.rawRequest(queryUrl)
		if err != nil {
			return nil, err
		}
		defer rawRespBody.Close()

		if decodeErr := jsoniter.NewDecoder(rawRespBody).Decode(&resp); decodeErr != nil {
			return nil, decodeErr
		}
		if resp.MaybeCode != nil && *resp.MaybeCode == ERR_CODE_ACCOUNT_NOT_FOUND {
			return nil, cosmosapp_interface.ErrAccountNotFound
		}
		if statusCode != 200 {
			return nil, fmt.Errorf("error requesting Cosmos %s endpoint: status code %d", queryUrl, statusCode)
		}

		for _, delegation := range resp.MaybeDelegationResponses {
			if delegation.Delegation.DelegatorAddress == delegator &&
				delegation.Delegation.ValidatorAddress == validator {
				client.httpCache.Set(cacheKey, delegation, utils.TIME_CACHE_FAST)
				return &delegation, nil
			}
		}

		if resp.MaybePagination.MaybeNextKey == nil {
			break
		}
	}

	return nil, nil
}

func (client *HTTPClient) AnnualProvisions() (coin.DecCoin, error) {
	cacheKey := "CosmosAnnualProvisions"
	var decCoinTmp coin.DecCoin

	err := client.httpCache.Get(cacheKey, &decCoinTmp)
	if err == nil {
		return decCoinTmp, nil
	}

	rawRespBody, err := client.request(client.getUrl("mint", "annual_provisions"))
	if err != nil {
		return coin.DecCoin{}, err
	}
	defer rawRespBody.Close()

	var annualProvisionsResp AnnualProvisionsResp
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&annualProvisionsResp); err != nil {
		return coin.DecCoin{}, err
	}

	annualProvisions, coinErr := coin.NewDecCoinFromString(client.bondingDenom, annualProvisionsResp.AnnualProvisions)
	if coinErr != nil {
		return coin.DecCoin{}, fmt.Errorf("error parsing coin from annual provision: %v", annualProvisions)
	}

	client.httpCache.Set(cacheKey, annualProvisions, utils.TIME_CACHE_FAST)

	return annualProvisions, nil
}

func (client *HTTPClient) TotalBondedBalance() (coin.Coin, error) {
	cacheKey := "CosmosTotalBondedBalance"
	var coinTmp coin.Coin

	err := client.httpCache.Get(cacheKey, &coinTmp)
	if err == nil {
		return coinTmp, nil
	}

	resp := &ValidatorsResp{
		MaybePagination: &Pagination{
			MaybeNextKey: nil,
			Total:        "",
		},
	}

	totalBondedBalance, newCoinErr := coin.NewCoin(client.bondingDenom, coin.ZeroInt())
	if newCoinErr != nil {
		return coin.Coin{}, fmt.Errorf("error when creating new coin: %v", newCoinErr)
	}
	for {
		queryUrl := client.getUrl("staking", "validators")
		if resp.MaybePagination.MaybeNextKey != nil {
			queryUrl = fmt.Sprintf(
				"%s?pagination.key=%s",
				queryUrl, url.QueryEscape(*resp.MaybePagination.MaybeNextKey),
			)
		}

		rawRespBody, statusCode, err := client.rawRequest(queryUrl)
		if err != nil {
			return coin.Coin{}, err
		}
		defer rawRespBody.Close()

		if decodeErr := jsoniter.NewDecoder(rawRespBody).Decode(&resp); decodeErr != nil {
			return coin.Coin{}, decodeErr
		}
		if resp.MaybeCode != nil && *resp.MaybeCode == ERR_CODE_ACCOUNT_NOT_FOUND {
			return coin.Coin{}, cosmosapp_interface.ErrAccountNotFound
		}
		if statusCode != 200 {
			return coin.Coin{}, fmt.Errorf("error requesting Cosmos %s endpoint: status code %d", queryUrl, statusCode)
		}

		for _, validator := range resp.MaybeValidatorResponse {
			bondedCoin, coinErr := coin.NewCoinFromString(client.bondingDenom, validator.Tokens)
			if coinErr != nil {
				if coinErr != nil {
					return coin.Coin{}, fmt.Errorf("error parsing Coin from validator tokens: %v", coinErr)
				}
			}
			totalBondedBalance = totalBondedBalance.Add(bondedCoin)
		}

		if resp.MaybePagination.MaybeNextKey == nil {
			break
		}
	}

	client.httpCache.Set(cacheKey, totalBondedBalance, utils.TIME_CACHE_FAST)

	return totalBondedBalance, nil
}

func (client *HTTPClient) Proposals() ([]cosmosapp_interface.Proposal, error) {
	cacheKey := "CosmosProposals"
	var proposalsTmp []cosmosapp_interface.Proposal

	err := client.httpCache.Get(cacheKey, &proposalsTmp)
	if err == nil {
		return proposalsTmp, nil
	}

	resp := &ProposalsResp{
		MaybePagination: &Pagination{
			MaybeNextKey: nil,
			Total:        "",
		},
	}

	proposals := make([]cosmosapp_interface.Proposal, 0)
	for {
		queryUrl := client.getUrl("gov", "proposals")
		if resp.MaybePagination.MaybeNextKey != nil {
			queryUrl = fmt.Sprintf(
				"%s?pagination.key=%s",
				queryUrl, url.QueryEscape(*resp.MaybePagination.MaybeNextKey),
			)
		}

		rawRespBody, statusCode, err := client.rawRequest(queryUrl)
		if err != nil {
			return nil, err
		}
		defer rawRespBody.Close()

		if decodeErr := jsoniter.NewDecoder(rawRespBody).Decode(&resp); decodeErr != nil {
			return nil, decodeErr
		}
		if statusCode != 200 {
			return nil, fmt.Errorf("error requesting Cosmos %s endpoint: status code %d", queryUrl, statusCode)
		}

		proposals = append(proposals, resp.MaybeProposalsResponse...)

		if resp.MaybePagination.MaybeNextKey == nil {
			break
		}
	}

	client.httpCache.Set(cacheKey, proposals, utils.TIME_CACHE_FAST)

	return proposals, nil
}

func (client *HTTPClient) ProposalById(id string) (cosmosapp_interface.Proposal, error) {
	cacheKey := fmt.Sprintf("CosmosProposalById_%s", id)
	var proposalTmp cosmosapp_interface.Proposal

	err := client.httpCache.Get(cacheKey, &proposalTmp)
	if err == nil {
		return proposalTmp, nil
	}

	method := fmt.Sprintf(
		"%s/%s",
		client.getUrl("gov", "proposals"), id,
	)
	rawRespBody, statusCode, err := client.rawRequest(
		method, "",
	)
	if err != nil {
		return cosmosapp_interface.Proposal{}, err
	}
	if statusCode == 404 {
		return cosmosapp_interface.Proposal{}, cosmosapp_interface.ErrProposalNotFound
	}
	if statusCode != 200 {
		return cosmosapp_interface.Proposal{}, fmt.Errorf("error requesting Cosmos %s endpoint: %d", method, statusCode)
	}
	defer rawRespBody.Close()

	var proposalResp ProposalResp
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&proposalResp); err != nil {
		return cosmosapp_interface.Proposal{}, err
	}
	client.httpCache.Set(cacheKey, proposalResp.Proposal, utils.TIME_CACHE_FAST)

	return proposalResp.Proposal, nil
}

func (client *HTTPClient) ProposalTally(id string) (cosmosapp_interface.Tally, error) {
	cacheKey := fmt.Sprintf("CosmosProposalTally_%s", id)
	var tallyTmp cosmosapp_interface.Tally

	err := client.httpCache.Get(cacheKey, &tallyTmp)
	if err == nil {
		return tallyTmp, nil
	}

	method := fmt.Sprintf(
		"%s/%s/tally",
		client.getUrl("gov", "proposals"), id,
	)
	rawRespBody, statusCode, err := client.rawRequest(
		method, "",
	)
	if err != nil {
		return cosmosapp_interface.Tally{}, err
	}
	if statusCode == 404 {
		return cosmosapp_interface.Tally{}, cosmosapp_interface.ErrProposalNotFound
	}
	if statusCode != 200 {
		return cosmosapp_interface.Tally{}, fmt.Errorf("error requesting Cosmos %s endpoint: %d", method, statusCode)
	}
	defer rawRespBody.Close()

	var tallyResp TallyResp
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&tallyResp); err != nil {
		return cosmosapp_interface.Tally{}, err
	}

	client.httpCache.Set(cacheKey, tallyResp.Tally, utils.TIME_CACHE_FAST)
	return tallyResp.Tally, nil
}

func (client *HTTPClient) DepositParams() (cosmosapp_interface.Params, error) {
	cacheKey := "CosmosDepositParams"
	var paramsTmp cosmosapp_interface.Params

	err := client.httpCache.Get(cacheKey, &paramsTmp)
	if err == nil {
		return paramsTmp, nil
	}

	method := fmt.Sprintf(
		"%s/%s/%s/%s/%s",
		"cosmos", "gov", "v1beta1", "params", "deposit",
	)
	rawRespBody, statusCode, err := client.rawRequest(
		method,
	)
	if err != nil {
		return cosmosapp_interface.Params{}, err
	}
	if statusCode == 404 {
		return cosmosapp_interface.Params{}, cosmosapp_interface.ErrProposalNotFound
	}
	if statusCode != 200 {
		return cosmosapp_interface.Params{}, fmt.Errorf("error requesting Cosmos %s endpoint: %d", method, statusCode)
	}
	defer rawRespBody.Close()

	var params cosmosapp_interface.Params
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&params); err != nil {
		return cosmosapp_interface.Params{}, err
	}

	client.httpCache.Set(cacheKey, params, utils.TIME_CACHE_FAST)

	return params, nil
}

func (client *HTTPClient) Tx(hash string) (*model.Tx, error) {
	cacheKey := fmt.Sprintf("CosmosTx_%s", hash)
	var txTmp *model.Tx

	err := client.httpCache.Get(cacheKey, &txTmp)
	if err == nil {
		return txTmp, nil
	}

	rawRespBody, err := client.request(
		fmt.Sprintf(
			"%s/%s",
			client.getUrl("tx", "txs"),
			hash,
		), "",
	)
	if err != nil {
		return nil, err
	}
	defer rawRespBody.Close()

	tx, err := ParseTxsResp(rawRespBody)
	if err != nil {
		return nil, fmt.Errorf("error parsing Tx(%s): %v", hash, err)
	}

	client.httpCache.Set(hash, tx, utils.TIME_CACHE_LONG)

	return tx, nil
}

func (client *HTTPClient) TotalFeeBurn() (cosmosapp_interface.TotalFeeBurn, error) {
	cacheKey := "CosmosTotalFeeBurn"
	var totalFeeBurnTmp cosmosapp_interface.TotalFeeBurn

	err := client.httpCache.Get(cacheKey, &totalFeeBurnTmp)
	if err == nil {
		return totalFeeBurnTmp, nil
	}

	method := fmt.Sprintf(
		"%s/%s/%s/%s",
		"astra", "feeburn", "v1", "total_fee_burn",
	)
	rawRespBody, statusCode, err := client.rawRequest(
		method,
	)
	if err != nil {
		return cosmosapp_interface.TotalFeeBurn{}, err
	}
	if statusCode == 404 {
		return cosmosapp_interface.TotalFeeBurn{}, cosmosapp_interface.ErrTotalFeeBurnNotFound
	}
	if statusCode != 200 {
		return cosmosapp_interface.TotalFeeBurn{}, fmt.Errorf("error requesting Cosmos %s endpoint: %d", method, statusCode)
	}
	defer rawRespBody.Close()

	var totalFeeBurn cosmosapp_interface.TotalFeeBurn
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&totalFeeBurn); err != nil {
		return cosmosapp_interface.TotalFeeBurn{}, err
	}

	client.httpCache.Set(cacheKey, totalFeeBurn, utils.TIME_CACHE_FAST)

	return totalFeeBurn, nil
}

func (client *HTTPClient) VestingBalances(account string) (cosmosapp_interface.VestingBalances, error) {
	cacheKey := fmt.Sprintf("CosmosVestingBalances_%s", account)
	var vestingBalancesTmp cosmosapp_interface.VestingBalances

	err := client.httpCache.Get(cacheKey, &vestingBalancesTmp)
	if err == nil {
		return vestingBalancesTmp, nil
	}

	method := fmt.Sprintf(
		"%s/%s/%s/%s/%s",
		"evmos", "vesting", "v1", "balances", account,
	)
	rawRespBody, statusCode, err := client.rawRequest(
		method,
	)

	vestingBalancesEmpty := cosmosapp_interface.VestingBalances{}
	vestingBalancesEmpty.Locked = []cosmosapp_interface.VestingBalance{}
	vestingBalancesEmpty.Unvested = []cosmosapp_interface.VestingBalance{}
	vestingBalancesEmpty.Vested = []cosmosapp_interface.VestingBalance{}

	if err != nil {
		return vestingBalancesEmpty, err
	}
	if statusCode == 404 {
		return vestingBalancesEmpty, cosmosapp_interface.ErrVestingBalancesNotFound
	}
	if statusCode != 200 {
		return vestingBalancesEmpty, fmt.Errorf("error requesting Cosmos %s endpoint: %d", method, statusCode)
	}
	defer rawRespBody.Close()

	var vestingBalances cosmosapp_interface.VestingBalances
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&vestingBalances); err != nil {
		return vestingBalancesEmpty, err
	}

	client.httpCache.Set(cacheKey, vestingBalances, utils.TIME_CACHE_FAST)

	return vestingBalances, nil
}

func (client *HTTPClient) VestingBalancesAsync(account string, vestingBalancesChan chan cosmosapp_interface.VestingBalances) {
	cacheKey := fmt.Sprintf("CosmosVestingBalancesAsync_%s", account)
	var vestingBalancesTmp cosmosapp_interface.VestingBalances

	err := client.httpCache.Get(cacheKey, &vestingBalancesTmp)
	if err == nil {
		vestingBalancesChan <- vestingBalancesTmp
		return
	}

	// Make sure we close these channels when we're done with them
	defer func() {
		close(vestingBalancesChan)
	}()

	method := fmt.Sprintf(
		"%s/%s/%s/%s/%s",
		"evmos", "vesting", "v1", "balances", account,
	)
	rawRespBody, statusCode, err := client.rawRequest(
		method,
	)

	vestingBalancesEmpty := cosmosapp_interface.VestingBalances{}
	vestingBalancesEmpty.Locked = []cosmosapp_interface.VestingBalance{}
	vestingBalancesEmpty.Unvested = []cosmosapp_interface.VestingBalance{}
	vestingBalancesEmpty.Vested = []cosmosapp_interface.VestingBalance{}

	if err != nil {
		vestingBalancesChan <- vestingBalancesEmpty
		return
	}
	if statusCode == 404 {
		vestingBalancesChan <- vestingBalancesEmpty
		return
	}
	if statusCode != 200 {
		vestingBalancesChan <- vestingBalancesEmpty
		return
	}
	defer rawRespBody.Close()

	var vestingBalances cosmosapp_interface.VestingBalances
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&vestingBalances); err != nil {
		vestingBalancesChan <- vestingBalancesEmpty
		return
	}

	client.httpCache.Set(cacheKey, vestingBalances, utils.TIME_CACHE_FAST)
	vestingBalancesChan <- vestingBalances
}

func (client *HTTPClient) BlockInfo(height string) (*cosmosapp_interface.BlockInfo, error) {
	cacheKey := fmt.Sprintf("CosmosBlockInfo_%s", height)

	var blockInfoTmp cosmosapp_interface.BlockInfo
	err := client.httpCache.Get(cacheKey, &blockInfoTmp)
	if err == nil {
		return &blockInfoTmp, nil
	}

	method := fmt.Sprintf(
		"%s/%s",
		"blocks", height,
	)
	rawRespBody, statusCode, err := client.rawRequest(
		method,
	)

	if err != nil {
		return nil, err
	}
	if statusCode == 404 {
		return nil, cosmosapp_interface.ErrBlockInfoNotFound
	}
	if statusCode != 200 {
		return nil, fmt.Errorf("error requesting Cosmos %s endpoint: %d", method, statusCode)
	}
	defer rawRespBody.Close()

	var blockInfo cosmosapp_interface.BlockInfo
	if err := jsoniter.NewDecoder(rawRespBody).Decode(&blockInfo); err != nil {
		return nil, err
	}

	if height != "latest" {
		client.httpCache.Set(cacheKey, &blockInfo, utils.TIME_CACHE_LONG)
	}
	return &blockInfo, nil
}

func ParseTxsResp(rawRespReader io.Reader) (*model.Tx, error) {
	var txsResp TxsResp
	if err := jsoniter.NewDecoder(rawRespReader).Decode(&txsResp); err != nil {
		return nil, err
	}

	height, err := strconv.ParseInt(txsResp.TxResponse.Height, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing txsResp.TxResponse.Height to int64 param: %v", err)
	}

	gasWanted, err := strconv.ParseInt(txsResp.TxResponse.GasWanted, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing txsResp.TxResponse.GasWanted to int64 param: %v", err)
	}

	gasUsed, err := strconv.ParseInt(txsResp.TxResponse.GasUsed, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing txsResp.TxResponse.GasUsed to int64 param: %v", err)
	}

	var tx = &model.Tx{
		Tx: txsResp.Tx,
		TxResponse: model.TxResponse{
			Height:    height,
			TxHash:    txsResp.TxResponse.TxHash,
			Codespace: txsResp.TxResponse.Codespace,
			Code:      txsResp.TxResponse.Code,
			Data:      txsResp.TxResponse.Data,
			RawLog:    txsResp.TxResponse.RawLog,
			Info:      txsResp.TxResponse.Info,
			GasWanted: gasWanted,
			GasUsed:   gasUsed,
			Timestamp: txsResp.TxResponse.Timestamp,
			Events:    txsResp.TxResponse.Events,
		},
	}

	if txsResp.TxResponse.Logs != nil {
		logs := make([]model.TxResponseLog, 0)
		for _, log := range txsResp.TxResponse.Logs {
			parsedLog := model.TxResponseLog{
				MsgIndex: log.MsgIndex,
				Log:      log.Log,
				Events:   log.Events,
			}
			logs = append(logs, parsedLog)
		}
		tx.TxResponse.Logs = logs
	}

	tx.TxResponse.Tx = model.TxResponseTx{
		Type:     txsResp.TxResponse.Tx.Type,
		CosmosTx: txsResp.TxResponse.Tx.CosmosTx,
	}

	return tx, nil
}

func (client *HTTPClient) getUrl(module string, method string) string {
	return fmt.Sprintf("cosmos/%s/v1beta1/%s", module, method)
}

// request construct tendermint getUrl and issues an HTTP request
// returns the success http Body
func (client *HTTPClient) request(method string, queryString ...string) (io.ReadCloser, error) {
	var err error
	startTime := time.Now()
	queryUrl := client.rpcUrl + "/" + method

	if len(queryString) > 0 {
		queryUrl += "?" + queryString[0]
	}

	req, err := retryablehttp.NewRequestWithContext(context.Background(), http.MethodGet, queryUrl, nil)
	if err != nil {
		prometheus.RecordApiExecTime(queryUrl, strconv.Itoa(408), "http", time.Since(startTime).Milliseconds())
		return nil, fmt.Errorf("error creating HTTP request with context: %v", err)
	}
	rawResp, err := client.httpClient.Do(req)
	if err != nil {
		prometheus.RecordApiExecTime(queryUrl, strconv.Itoa(400), "http", time.Since(startTime).Milliseconds())
		return nil, fmt.Errorf("error requesting Cosmos %s endpoint: %v", queryUrl, err)
	}

	prometheus.RecordApiExecTime(queryUrl, strconv.Itoa(rawResp.StatusCode), "http", time.Since(startTime).Milliseconds())

	if rawResp.StatusCode != 200 {
		rawResp.Body.Close()
		return nil, fmt.Errorf("error requesting Cosmos %s endpoint: %s", method, rawResp.Status)
	}

	return rawResp.Body, nil
}

// rawRequest construct tendermint getUrl and issues an HTTP request
// returns the http Body with any status code
func (client *HTTPClient) rawRequest(method string, queryString ...string) (io.ReadCloser, int, error) {
	var err error
	startTime := time.Now()
	queryUrl := client.rpcUrl + "/" + method
	if len(queryString) > 0 {
		queryUrl += "?" + queryString[0]
	}

	req, err := retryablehttp.NewRequestWithContext(context.Background(), http.MethodGet, queryUrl, nil)
	if err != nil {
		prometheus.RecordApiExecTime(method, strconv.Itoa(408), "http", time.Since(startTime).Milliseconds())
		return nil, 0, fmt.Errorf("error creating HTTP request with context: %v", err)
	}
	// nolint:bodyclose
	rawResp, err := client.httpClient.Do(req)
	if err != nil {
		prometheus.RecordApiExecTime(method, strconv.Itoa(400), "http", time.Since(startTime).Milliseconds())
		return nil, 0, fmt.Errorf("error requesting Cosmos %s endpoint: %v", queryUrl, err)
	}
	prometheus.RecordApiExecTime(method, strconv.Itoa(rawResp.StatusCode), "http", time.Since(startTime).Milliseconds())
	return rawResp.Body, rawResp.StatusCode, nil
}

type Pagination struct {
	MaybeNextKey *string `json:"next_key"`
	Total        string  `json:"total"`
}
