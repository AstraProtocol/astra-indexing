package cosmosapp

import cosmosapp_interface "github.com/AstraProtocol/astra-indexing/appinterface/cosmosapp"

type AccountResp struct {
	Account Account
}

type Account struct {
	// Common fields
	Type string `json:"@type"`

	// Module account
	MaybeName        *string      `json:"name"`
	MaybeBaseAccount *BaseAccount `json:"base_account"`
	MaybePermissions []string     `json:"permissions"`

	// Vesting account common fields
	MaybeBaseVestingAccount *BaseVestingAccount `json:"base_vesting_account"`
	// Continuous vesting account
	MaybeStartTime     *string `json:"start_time"`
	MaybeFunderAddress *string `json:"funder_address"`
	// Periodic vesting account
	MaybeVestingPeriods []cosmosapp_interface.VestingPeriod `json:"vesting_periods"`
	MaybeLockupPeriods  []cosmosapp_interface.VestingPeriod `json:"lockup_periods"`

	// User account
	MaybeAddress       *string                     `json:"address"`
	MaybePubKey        *cosmosapp_interface.PubKey `json:"pub_key"`
	MaybeAccountNumber *string                     `json:"account_number"`
	MaybeSequence      *string                     `json:"sequence"`
}

type BaseVestingAccount struct {
	BaseAccount      BaseAccount                   `json:"base_account"`
	OriginalVesting  []cosmosapp_interface.Balance `json:"original_vesting"`
	DelegatedFree    []cosmosapp_interface.Balance `json:"delegated_free"`
	DelegatedVesting []cosmosapp_interface.Balance `json:"delegated_vesting"`
	EndTime          string                        `json:"end_time"`
}

type BaseAccount struct {
	Address       string                      `json:"address"`
	MaybePubKey   *cosmosapp_interface.PubKey `json:"pub_key"`
	AccountNumber string                      `json:"account_number"`
	Sequence      string                      `json:"sequence"`
}

type BankBalancesResp struct {
	BankBalanceResponses []BankBalance `json:"balances"`
	Pagination           Pagination    `json:"pagination"`
}

type BankBalance struct {
	Amount string `json:"amount"`
	Denom  string `json:"denom"`
}
