package view

import (
	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
)

type AccountMessagesTotal interface {
	Set(string, int64) error
	Increment(string, int64) error
	IncrementAll([]string, int64) error
	DecrementAll([]string, int64) error
	FindBy(string) (int64, error)
	SumBy([]string) (int64, error)
}

type AccountMessagesTotalView struct {
	*view.Total
}

func NewAccountMessagesTotalView(rdbHandle *rdb.Handle) AccountMessagesTotal {
	return &AccountMessagesTotalView{
		view.NewTotal(rdbHandle, "view_account_messages_total"),
	}
}
