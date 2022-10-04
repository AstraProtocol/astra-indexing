package cache

import (
	"github.com/AstraProtocol/astra-indexing/external/utctime"
	"github.com/AstraProtocol/astra-indexing/projection/block/view"

	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSet(t *testing.T) {
	cache := NewCache()
	err := cache.Set("123", "nguyen khanh hoa", 100)
	assert.Equal(t, nil, err)

	output := ""
	err = cache.Get("123", &output)
	assert.Equal(t, nil, err)
	assert.Equal(t, output, "nguyen khanh hoa")
}

func TestCacheBlock(t *testing.T) {
	cache := NewCache()
	block := view.Block{
		Height:                123,
		Hash:                  "123",
		Time:                  utctime.UTCTime{},
		AppHash:               "",
		TransactionCount:      0,
		CommittedCouncilNodes: nil,
	}
	//pr := pagination.NewOffsetPagination(0, 0)
	//blockResult := handlers.BlocksPaginationResult{
	//	[]view.Block{
	//	},
	//	*pr,
	//}
	err := cache.Set("123", block, 100)
	assert.Equal(t, nil, err)

	output := view.Block{}
	err = cache.Get("123", &output)
	assert.Equal(t, nil, err)

	assert.Equal(t, output, block)
}
