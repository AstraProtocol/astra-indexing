package blockscout

type SourceCodeResp struct {
	Message string             `json:"message"`
	Result  []SourceCodeResult `json:"result"`
	Status  string             `json:"status"`
}

type SourceCodeResult struct {
	Abi                   string `json:"ABI"`
	CompilerVersion       string `json:"CompilerVersion"`
	ContractName          string `json:"ContractName"`
	FileName              string `json:"FileName"`
	ImplementationAddress string `json:"ImplementationAddress"`
	IsProxy               string `json:"IsProxy"`
	OptimizationUsed      string `json:"OptimizationUsed"`
	SourceCode            string `json:"SourceCode"`
}
