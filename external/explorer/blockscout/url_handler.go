package blockscout

import (
	"fmt"
	"sync"
)

const GET_DETAIL_EVM_TX = "/api/v1?module=transaction&action=getTxCosmosInfo&txhash="

type BlockscoutUrl struct {
	blockscoutUrl string
}

var singleInstance *BlockscoutUrl
var once sync.Once

func InitSingleton(url string) {
	if singleInstance == nil {
		once.Do(
			func() {
				singleInstance = &BlockscoutUrl{
					url,
				}
			})
	}
}

func GetInstance() *BlockscoutUrl {
	if singleInstance == nil {
		panic("Attempting to retrieve uninitialized BlockscoutUrl singleton")
	}
	return singleInstance
}

func (url *BlockscoutUrl) GetDetailEvmTxUrl(txHash string) string {
	endpoint := fmt.Sprintf("%s%s%s", url.blockscoutUrl, GET_DETAIL_EVM_TX, txHash)
	return endpoint
}
