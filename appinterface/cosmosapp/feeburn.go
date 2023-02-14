package cosmosapp

type TotalFeeBurn struct {
	TotalFeeBurn BurnAmount `json:"total_fee_burn"`
}

type BurnAmount struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}
