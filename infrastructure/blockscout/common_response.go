package blockscout

type CommonResp struct {
	Message string      `json:"message"`
	Result  interface{} `json:"result"`
	Status  string      `json:"status"`
}

type CommonPaginationResp struct {
	HasNextPage  bool        `json:"hasNextPage"`
	NextPagePath string      `json:"nextPagePath"`
	Result       interface{} `json:"result"`
}
