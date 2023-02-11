package model

type MsgClawbackParams struct {
	RawMsgClawback
}

type RawMsgClawback struct {
	Type           string `mapstructure:"@type" json:"@type"`
	FunderAddress  string `mapstructure:"funder_address" json:"funder_address"`
	AccountAddress string `mapstructure:"account_address" json:"account_address"`
	DestAddress    string `mapstructure:"dest_address" json:"dest_address"`
}
