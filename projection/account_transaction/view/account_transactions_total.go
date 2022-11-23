package view

import (
	"fmt"

	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
)

type AccountTransactionsTotal struct {
	*view.Total
}

func NewAccountTransactionsTotal(rdbHandle *rdb.Handle) *AccountTransactionsTotal {
	return &AccountTransactionsTotal{
		view.NewTotal(rdbHandle, "view_account_transactions_total"),
	}
}

func (total *AccountTransactionsTotal) Search(address string) (bool, error) {
	numberOfRowsFound, err := total.FindBy(fmt.Sprintf("%s:-", address))
	if err != nil {
		return false, err
	}
	if numberOfRowsFound == 0 {
		return false, nil
	}
	return true, nil
}
