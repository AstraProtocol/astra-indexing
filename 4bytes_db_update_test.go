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

	util.UpdateSignature("0x743cec2d", "exchangeWithValue(uint256[], uint256, uint256, bytes)")

	res, err := util.GetSignature("0xa07aea1c")
	if err != nil {
		panic(err)
	}
	assert.Equal(res, "addOperators(address[])")

	input := "0x743cec2d0000421400000000"
	assert.Equal(util.GetMethodNameFromMethodId(input[2:10]), "exchangeWithValue")

	data := "0q7S1gAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAGSWzhIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAoAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACcAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQVbZvQpCuU8VUVDSeyAd243bdr5ljiPA3bK7aUm/VuQVdhajtudaHAI7ltLe1GHQeZchiXxaXAq2Ez+6ErCpJRscAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="
	signature := util.GetMethodNameFromData(data)
	assert.Equal(signature, "exchange")

	data = "dDzsLQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAGTEB/EAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA3gtrOnZAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAmAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABB/dVhop2u7EJPGDDbEUpNbTY4eI4Wr932AATbuG+lk0sBtEXv6FsIn1g1ZP9D3/E/0Ne0ktW5lqb5N2cIcX7fgRsAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	signature = util.GetMethodNameFromData(data)
	assert.Equal(signature, "exchangeWithValue")
}
