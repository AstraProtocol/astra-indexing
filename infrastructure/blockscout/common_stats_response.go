package blockscout

type TokenStats struct {
	CirculatingSupply string `json:"circulating_supply"`
	MarketCap         string `json:"market_cap"`
	Price             string `json:"price"`
	Volume24h         string `json:"volume_24h"`
}

type TransactionStats struct {
	Date                 string `json:"date"`
	GasUsed              string `json:"gas_used"`
	NumberOfTransactions int    `json:"number_of_transactions"`
	TotalFee             string `json:"total_fee"`
}

type CommonStats struct {
	AverageBlockTime float64          `json:"average_block_time"`
	TokenStats       TokenStats       `json:"token_stats"`
	TransactionStats TransactionStats `json:"transaction_stats"`
}