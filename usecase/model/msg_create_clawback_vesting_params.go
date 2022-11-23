package model

import (
	"github.com/AstraProtocol/astra-indexing/usecase/coin"
	"time"
)

type MsgCreateClawbackVestingAccountParams struct {
	RawMsgCreateClawbackVestingAccount
}

type RawMsgCreateClawbackVestingAccount struct {
	Type           string          `mapstructure:"@type" json:"@type"`
	FromAddress    string          `mapstructure:"from_address" json:"from_address"`
	ToAddress      string          `mapstructure:"to_address" json:"to_address"`
	StartTime      time.Time       `mapstructure:"start_time" json:"start_time"`
	LockupPeriods  []VestingPeriod `mapstructure:"lockup_periods" json:"lockup_periods"`
	VestingPeriods []VestingPeriod `mapstructure:"vesting_periods" json:"vesting_periods"`
	Merge          bool            `mapstructure:"merge" json:"merge"`
}

type VestingPeriod struct {
	Amount []coin.Coin `json:"amount"`
	Length string      `json:"length"`
}
