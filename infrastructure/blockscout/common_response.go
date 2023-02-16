package blockscout

type CommonResp struct {
	Message string      `json:"message"`
	Result  interface{} `json:"result"`
	Status  string      `json:"status"`
}
