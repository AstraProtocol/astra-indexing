package cosmosapp

type PoolParams struct {
	NotBondedTokens string `json:"not_bonded_tokens"`
	BondedTokens    string `json:"bonded_tokens"`
}

type StakingPool struct {
	Pool PoolParams `json:"pool"`
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
