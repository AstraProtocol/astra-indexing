package cache

import "time"

type CacheEngine interface {
	Get(key string, value interface{}) error
	Set(key string, value interface{}, expireAt time.Duration) error
}
