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

type MarketHistory struct {
	HistoryData string `json:"history_data"`
	SupplyData  string `json:"supply_data"`
}

type GasPriceOracle struct {
	Average float64 `json:"average"`
	Fast    float64 `json:"fast"`
	Slow    float64 `json:"slow"`
}

type CoinBalancesByDate struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}
