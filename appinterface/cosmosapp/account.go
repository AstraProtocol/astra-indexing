package cosmosapp

const ACCOUNT_MODULE = "/cosmos.auth.v1beta1.ModuleAccount"
const ACCOUNT_BASE = "/cosmos.auth.v1beta1.BaseAccount"
const ACCOUNT_VESTING_DELAYED = "/cosmos.vesting.v1beta1.DelayedVestingAccount"
const ACCOUNT_VESTING_CONTINUOUS = "/cosmos.vesting.v1beta1.ContinuousVestingAccount"
const ACCOUNT_VESTING_PERIODIC = "/cosmos.vesting.v1beta1.PeriodicVestingAccount"
const ACCOUNT_ETHERMINT = "/ethermint.types.v1.EthAccount"
const ACCOUNT_CLAWBACK_VESTING = "/evmos.vesting.v1.ClawbackVestingAccount"

type Account struct {
	Type          string  `json:"type"`
	Address       string  `json:"address"`
	MaybePubkey   *PubKey `json:"pubkey"`
	AccountNumber string  `json:"account_number"`
	Sequence      string  `json:"sequence"`

	MaybeModuleAccount            *ModuleAccount            `json:"module_account"`
	MaybeDelayedVestingAccount    *DelayedVestingAccount    `json:"delayed_vesting_account"`
	MaybeContinuousVestingAccount *ContinuousVestingAccount `json:"continuous_vesting_account"`
	MaybePeriodicVestingAccount   *PeriodicVestingAccount   `json:"periodic_vesting_account"`
	MaybeClawbackVestingAccount   *ClawbackVestingAccount   `json:"clawback_vesting_account"`
}

type ModuleAccount struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

type DelayedVestingAccount struct {
	OriginalVesting  []Balance `json:"original_vesting"`
	DelegatedFree    []Balance `json:"delegated_free"`
	DelegatedVesting []Balance `json:"delegated_vesting"`
	EndTime          string    `json:"end_time"`
}

type ContinuousVestingAccount struct {
	OriginalVesting  []Balance `json:"original_vesting"`
	DelegatedFree    []Balance `json:"delegated_free"`
	DelegatedVesting []Balance `json:"delegated_vesting"`
	StartTime        string    `json:"start_time"`
	EndTime          string    `json:"end_time"`
}

type PeriodicVestingAccount struct {
	OriginalVesting  []Balance       `json:"original_vesting"`
	DelegatedFree    []Balance       `json:"delegated_free"`
	DelegatedVesting []Balance       `json:"delegated_vesting"`
	StartTime        string          `json:"start_time"`
	EndTime          string          `json:"end_time"`
	VestingPeriods   []VestingPeriod `json:"vesting_periods"`
}

type VestingPeriod struct {
	Amount []Balance `json:"amount"`
	Length string    `json:"length"`
}

type Balance struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}

type ClawbackVestingAccount struct {
	FunderAddress  string          `json:"funder_address"`
	StartTime      string          `json:"start_time"`
	LockupPeriod   []VestingPeriod `json:"lockup_period"`
	VestingPeriods []VestingPeriod `json:"vesting_periods"`
}

type VestingBalances struct {
	Locked   []Balance `json:"locked"`
	Unvested []Balance `json:"unvested"`
	Vested   []Balance `json:"vested"`
}
