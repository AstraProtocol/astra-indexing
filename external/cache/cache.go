package cache

import (
	"encoding/json"
	"fmt"
	"github.com/jellydator/ttlcache/v3"
	"time"
)

type AstraCache struct {
	astraCache *ttlcache.Cache[string, []byte]
}

func NewCache() *AstraCache {
	cache := ttlcache.New[string, []byte]()
	return &AstraCache{astraCache: cache}
}

func (ac *AstraCache) Set(key string, value interface{}, expireAt time.Duration) error {
	tmpValue, err := json.Marshal(value)
	if err != nil {
		return err
	}
	ac.astraCache.Set(key, tmpValue, expireAt)
	return nil
}

func (ac *AstraCache) Get(key string, valueOutput interface{}) error {
	tmpData := ac.astraCache.Get(key)
	if tmpData != nil {
		return json.Unmarshal(tmpData.Value(), &valueOutput)
	}
	return fmt.Errorf("not found")
}
