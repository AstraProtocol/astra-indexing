package evm

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test4Bytes(t *testing.T) {
	util, err := NewEvmUtils()
	if err != nil {
		panic(err)
	}

	//0xc0331b3e
	res, err := util.GetSignature("0xc0331b3e")
	if err != nil {
		panic(err)
	}
	fmt.Println("hoank", res)
}

func TestUpdate4Bytes(t *testing.T) {
	assert := assert.New(t)

	util, err := NewEvmUtils()
	if err != nil {
		panic(err)
	}

	util.UpdateSignature("0xd2aed2d6", "exchange(uint256[],uint256,bytes)")
	util.UpdateSignature("0x1482dcda", "mintCoupons(address,uint256[])")
	util.UpdateSignature("0xc4237db1", "createRewardProg(address,bytes)")
	util.UpdateSignature("0x2b0e3b52", "createCouponProg(address,bytes)")
	util.UpdateSignature("0x7a1b5ca3", "createRoleContract(bytes)")
	util.UpdateSignature("0x3b0f6c11", "initQRDiscountProgTemplate(address,uint16,address)")
	util.UpdateSignature("0xf2ac0d12", "initRoleTemplate(address,uint16)")
	util.UpdateSignature("0xbfc05b8a", "initCouponProgTemplate(address,uint16,address)")
	util.UpdateSignature("0xe9fdccd0", "initRewardProgTemplate(address,uint16,address)")
	util.UpdateSignature("0xb134a366", "grantLockedReward(address[],uint256[],uint256[])")
	util.UpdateSignature("0xf3fef3a3", "withdraw(address,uint256)")
	util.UpdateSignature("0xa07aea1c", "addOperators(address[])")

	res, err := util.GetSignature("0xbe739356")
	if err != nil {
		panic(err)
	}
	assert.Equal(res, "sendReward(address[],uint256[])")
}

func Test4GetSignatureFromData(t *testing.T) {
	assert := assert.New(t)

	util, err := NewEvmUtils()
	if err != nil {
		panic(err)
	}
	data := "0q7S1gAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAGSWzhIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAoAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACcAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQVbZvQpCuU8VUVDSeyAd243bdr5ljiPA3bK7aUm/VuQVdhajtudaHAI7ltLe1GHQeZchiXxaXAq2Ez+6ErCpJRscAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="
	signature := util.GetSignatureFromData(data)
	assert.Equal(signature, "exchange")

	data = "sTSjZgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAKAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA4AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAqJyv7Z/fSgPMmUkRT3r1XgqKDY4AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAiGyYt2AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAZHm5ZA=="
	signature = util.GetSignatureFromData(data)
	assert.Equal(signature, "grantLockedReward")
}
