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
