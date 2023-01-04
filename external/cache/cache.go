package cache

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"
	"github.com/jellydator/ttlcache/v3"
)

type AstraCache struct {
	astraCache *ttlcache.Cache[string, []byte]
}

func NewCache(appName string) *AstraCache {
	cache := ttlcache.New[string, []byte]()
	go cache.Start() // starts automatic expired item deletion
	go func() {
		for {
			metrics := cache.Metrics()
			prometheus.RecordCacheMissed(appName, strconv.FormatUint(metrics.Misses, 10))
			prometheus.RecordCacheInsertion(appName, strconv.FormatUint(metrics.Insertions, 10))
			prometheus.RecordCacheHits(appName, strconv.FormatUint(metrics.Hits, 10))
			prometheus.RecordCacheEviction(appName, strconv.FormatUint(metrics.Evictions, 10))
			time.Sleep(2 * time.Second)
		}
	}()
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
