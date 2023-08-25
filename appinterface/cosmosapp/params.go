package cosmosapp

type VotingParams struct {
	Params VotingPeriod `json:"voting_params"`
}

type VotingPeriod struct {
	Period string `json:"voting_period"`
}

type DepositParams struct {
	MinDeposit       []TotalDeposit `json:"min_deposit"`
	MaxDepositPeriod string         `json:"max_deposit_period"`
}

type TallyParams struct {
	Quorum        string `json:"quorum"`
	Threshold     string `json:"threshold"`
	VetoThreshold string `json:"veto_threshold"`
}

type Params struct {
	VotingParams  VotingParams  `json:"voting_params"`
	DepositParams DepositParams `json:"deposit_params"`
	TallyParams   TallyParams   `json:"tally_params"`
}

type Staking struct {
	UnbondingTime     string `json:"unbonding_time"`
	MaxValidators     uint   `json:"max_validators"`
	MaxEntries        uint   `json:"max_entries"`
	HistoricalEntries uint   `json:"historical_entries"`
	BondDenom         string `json:"bond_denom"`
	MinCommissionRate string `json:"min_commission_rate"`
}

type StakingParams struct {
	Params Staking `json:"params"`
}

type FeeMarket struct {
	NoBaseFee                bool   `json:"no_base_fee"`
	BaseFeeChangeDenominator uint64 `json:"base_fee_change_denominator"`
	ElasticityMultiplier     uint   `json:"elasticity_multiplier"`
	EnableHeight             string `json:"enable_height"`
	BaseFee                  string `json:"base_fee"`
	MinGasPrice              string `json:"min_gas_price"`
	MinGasMultiplier         string `json:"min_gas_multiplier"`
}

type FeeParams struct {
	Params FeeMarket `json:"params"`
}
