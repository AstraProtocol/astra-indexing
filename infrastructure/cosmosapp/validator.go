package cosmosapp

import cosmosapp_interface "github.com/AstraProtocol/astra-indexing/appinterface/cosmosapp"

type ValidatorsResp struct {
	MaybeValidatorResponse []cosmosapp_interface.Validator `json:"validators"`
	MaybePagination        *Pagination                     `json:"pagination"`
	// On error
	MaybeCode    *int    `json:"code"`
	MaybeMessage *string `json:"message"`
}

type ValidatorResp struct {
	Validator cosmosapp_interface.Validator `json:"validator"`
}
