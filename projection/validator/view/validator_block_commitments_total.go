package view

import (
	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
)

type ValidatorBlockCommitmentsTotal struct {
	*view.Total
}

func NewValidatorBlockCommitmentsTotal(rdbHandle *rdb.Handle) *ValidatorBlockCommitmentsTotal {
	return &ValidatorBlockCommitmentsTotal{
		view.NewTotal(rdbHandle, "view_validator_block_commitments_total"),
	}
}
