package view

import (
	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
)

type TransactionsTotal interface {
	Set(string, int64) error
	Increment(string, int64) error
	IncrementAll([]string, int64) error
	DecrementAll([]string, int64) error
	FindBy(string) (int64, error)
	SumBy([]string) (int64, error)
}

type TransactionsTotalView struct {
	*view.Total
}

func NewTransactionsTotalView(rdbHandle *rdb.Handle) TransactionsTotal {
	return &TransactionsTotalView{
		view.NewTotal(rdbHandle, "view_transactions_total"),
	}
}
