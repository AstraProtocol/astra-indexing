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

	util.UpdateSignature("0xbe739356", "sendReward(address[] to, uint256[] amounts)")
	util.UpdateSignature("0xd2aed2d6", "exchange(uint256[] tokenIds, uint256 deadline, bytes exchangeSig)")
	util.UpdateSignature("0x1482dcda", "mintCoupons(address to, uint256[] tokenIds)")
	util.UpdateSignature("0xc4237db1", "createRewardProg(address roleContract, bytes data)")
	util.UpdateSignature("0x2b0e3b52", "createCouponProg(address roleContract, bytes data)")
	util.UpdateSignature("0x7a1b5ca3", "createRoleContract(bytes data)")
	util.UpdateSignature("0x3b0f6c11", "initQRDiscountProgTemplate(address program, uint16 version, address _role)")
	util.UpdateSignature("0xf2ac0d12", "initRoleTemplate(address role, uint16 version)")
	util.UpdateSignature("0xbfc05b8a", "initCouponProgTemplate(address program, uint16 version, address _role)")
	util.UpdateSignature("0xe9fdccd0", "initRewardProgTemplate(address program, uint16 version, address _role)")
	util.UpdateSignature("0xb134a366", "grantLockedReward(address[] _holders, uint256[] _amount, uint256[] _expireTime)")
	util.UpdateSignature("0xf3fef3a3", "withdraw(address to, uint256 amount)")
	util.UpdateSignature("0xa07aea1c", "addOperators(address[] operators)")

	res, err := util.GetSignature("0xbe739356")
	if err != nil {
		panic(err)
	}
	assert.Equal(res, "sendReward(address[] to, uint256[] amounts)")
}
