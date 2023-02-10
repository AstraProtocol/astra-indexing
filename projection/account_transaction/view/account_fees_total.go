package view

import (
	"encoding/hex"

	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/external/tmcosmosutils"
	"github.com/AstraProtocol/astra-indexing/internal/evm"
)

type AccountFeesTotal struct {
	*view.Total
}

func NewAccountFeesTotal(rdbHandle *rdb.Handle) *AccountFeesTotal {
	return &AccountFeesTotal{
		view.NewTotal(rdbHandle, "view_account_fees_total"),
	}
}

func (total *AccountFeesTotal) Search(address string) (bool, error) {
	if tmcosmosutils.IsValidCosmosAddress(address) {
		_, converted, _ := tmcosmosutils.DecodeAddressToHex(address)
		address = "0x" + hex.EncodeToString(converted)
	} else {
		if !evm.IsHexAddress(address) {
			return false, nil
		}
	}
	numberOfRowsFound, err := total.FindBy(address)
	if err != nil {
		return false, err
	}
	if numberOfRowsFound == 0 {
		return false, nil
	}
	return true, nil
}
