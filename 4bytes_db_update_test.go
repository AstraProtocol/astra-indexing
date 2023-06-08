package main

import (
	"testing"

	"github.com/AstraProtocol/astra-indexing/internal/evm"
	"github.com/stretchr/testify/assert"
)

func TestUpdate4Bytes(t *testing.T) {
	assert := assert.New(t)

	util, err := evm.NewEvmUtils()
	if err != nil {
		panic(err)
	}

	util.UpdateSignature("0xde76c394", "addDiscountOperators(address[])")
	util.UpdateSignature("0x110f3814", "addRewardOperators(address[])")
	util.UpdateSignature("0x448e8250", "setProgramName(string)")

	res, err := util.GetSignature("0xa07aea1c")
	if err != nil {
		panic(err)
	}
	assert.Equal(res, "addOperators(address[])")

	data := "0q7S1gAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAGSWzhIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAoAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACcAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQVbZvQpCuU8VUVDSeyAd243bdr5ljiPA3bK7aUm/VuQVdhajtudaHAI7ltLe1GHQeZchiXxaXAq2Ez+6ErCpJRscAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="
	signature := util.GetSignatureFromData(data)
	assert.Equal(signature, "exchange")

	data = "RI6CUAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABdlVm91Y2hlciBHcmFiIDMwLjAwMCDEkQAAAAAAAAAAAA=="
	signature = util.GetSignatureFromData(data)
	assert.Equal(signature, "setProgramName")
}
