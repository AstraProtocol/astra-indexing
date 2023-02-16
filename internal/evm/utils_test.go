package evm

import (
	"fmt"
	"testing"
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
