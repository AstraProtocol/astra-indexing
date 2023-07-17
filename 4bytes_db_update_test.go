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

	util.UpdateSignature("0x18cbafe5", "swapExactTokensForETH(uint256, uint256, address[], address, uint256)")
	util.UpdateSignature("0x095ea7b3", "approve(address, uint256)")
	util.UpdateSignature("0xe2bbb158", "deposit(uint256, uint256)")
	util.UpdateSignature("0xded9382a", "removeLiquidityETHWithPermit(address, uint256, uint256, uint256, address, uint256, bool, uint8, bytes32, bytes32)")
	util.UpdateSignature("0xa9059cbb", "transfer(address, uint256)")
	util.UpdateSignature("0x441a3e70", "withdraw(uint256, uint256)")
	util.UpdateSignature("0x71679309", "zapInEth(address, address, address, uint256, uint256)")
	util.UpdateSignature("0xf305d719", "addLiquidityETH(address, uint256, uint256, uint256, address, uint256)")
	util.UpdateSignature("0x2195995c", "removeLiquidityWithPermit(address, address, uint256, uint256, uint256, address, uint256, bool, uint8, bytes32, bytes32)")
	util.UpdateSignature("0xb6f9de95", "swapExactETHForTokensSupportingFeeOnTransferTokens(uint256, address[], address, uint256)")
	util.UpdateSignature("0x38ed1739", "swapExactTokensForTokens(uint256, uint256, address[], address, uint256)")
	util.UpdateSignature("0xe8e33700", "addLiquidity(address, address, uint256, uint256, uint256, uint256, address, uint256)")

	res, err := util.GetSignature("0xa07aea1c")
	if err != nil {
		panic(err)
	}
	assert.Equal(res, "addOperators(address[])")

	input := "0x18cbafe5"
	assert.Equal(util.GetMethodNameFromMethodId(input[2:10]), "swapExactTokensForETH")
	assert.Equal(util.GetMethodNameFromMethodId("0x095ea7b3"), "approve")
	assert.Equal(util.GetMethodNameFromMethodId("0xe2bbb158"), "deposit")
	assert.Equal(util.GetMethodNameFromMethodId("0xded9382a"), "removeLiquidityETHWithPermit")
	assert.Equal(util.GetMethodNameFromMethodId("0xa9059cbb"), "transfer")
	assert.Equal(util.GetMethodNameFromMethodId("0x441a3e70"), "withdraw")
	assert.Equal(util.GetMethodNameFromMethodId("0x71679309"), "zapInEth")
	assert.Equal(util.GetMethodNameFromMethodId("0xf305d719"), "addLiquidityETH")
	assert.Equal(util.GetMethodNameFromMethodId("0x2195995c"), "removeLiquidityWithPermit")
	assert.Equal(util.GetMethodNameFromMethodId("0xb6f9de95"), "swapExactETHForTokensSupportingFeeOnTransferTokens")
	assert.Equal(util.GetMethodNameFromMethodId("0x38ed1739"), "swapExactTokensForTokens")
	assert.Equal(util.GetMethodNameFromMethodId("0xe8e33700"), "addLiquidity")

	data := "0q7S1gAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAGSWzhIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAoAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACcAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQVbZvQpCuU8VUVDSeyAd243bdr5ljiPA3bK7aUm/VuQVdhajtudaHAI7ltLe1GHQeZchiXxaXAq2Ez+6ErCpJRscAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="
	signature := util.GetMethodNameFromData(data)
	assert.Equal(signature, "exchange")
}
