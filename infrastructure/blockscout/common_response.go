package blockscout

type CommonResp struct {
	Message string      `json:"message"`
	Result  interface{} `json:"result"`
	Status  string      `json:"status"`
}

type CommonPaginationResp struct {
	HasNextPage  bool        `json:"hasNextPage"`
	Result       interface{} `json:"result"`
	NextPagePath string      `json:"nextPagePath"`
}
