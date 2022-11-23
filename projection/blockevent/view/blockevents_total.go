package view

import (
	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
)

type BlockEventsTotal struct {
	*view.Total
}

func NewBlockEventsTotal(rdbHandle *rdb.Handle) *BlockEventsTotal {
	return &BlockEventsTotal{
		view.NewTotal(rdbHandle, "view_block_events_total"),
	}
}
