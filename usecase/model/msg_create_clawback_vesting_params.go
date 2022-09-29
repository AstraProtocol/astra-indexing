package model

import (
	"time"
)

type MsgCreateClawbackVestingAccountParams struct {
	RawMsgCreateClawbackVestingAccount
}

type RawMsgCreateClawbackVestingAccount struct {
	Type        string    `mapstructure:"@type" json:"@type"`
	FromAddress string    `mapstructure:"from_address" json:"from_address"`
	ToAddress   string    `mapstructure:"to_address" json:"to_address"`
	StartTime   time.Time `mapstructure:"start_time" json:"start_time"`
	Merge       bool      `mapstructure:"merge" json:"merge"`
}
