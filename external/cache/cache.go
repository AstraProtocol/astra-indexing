package cache

import (
	"encoding/json"
	"github.com/coocood/freecache"
)

type AstraCache struct {
	astraCache *freecache.Cache
}

func NewCache() *AstraCache {
	cacheSize := 1024 * 1024 * 1024 // 50 MB
	cache := freecache.NewCache(cacheSize)
	return &AstraCache{astraCache: cache}
}

func (ac *AstraCache) Set(key string, value interface{}, expireAt int) error {
	tmpValue, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return ac.astraCache.Set([]byte(key), tmpValue, expireAt)
}

func (ac *AstraCache) Get(key string, valueOutput interface{}) error {
	tmpData, err := ac.astraCache.Get([]byte(key))
	if err != nil {
		return err
	}
	return json.Unmarshal(tmpData, &valueOutput)
}
