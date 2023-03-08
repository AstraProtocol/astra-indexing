package cache

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/eko/gocache/lib/v4/store"
	redis_store "github.com/eko/gocache/store/redis/v4"
	"github.com/go-redis/redis/v8"
)

type AstraCache struct {
	astraCache *redis_store.RedisStore
}

func NewCache() *AstraCache {
	redisURL, ok := os.LookupEnv("REDIS_URL")

	if !ok {
		log.Fatalln("Missing REDIS_URL string")
	}

	opt, _ := redis.ParseURL(redisURL)
	redisStore := redis_store.NewRedis(redis.NewClient(opt))
	return &AstraCache{astraCache: redisStore}
}

func (ac *AstraCache) Set(key string, value interface{}, expireAt time.Duration) error {
	tmpValue, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return ac.astraCache.Set(context.Background(), key, tmpValue, store.WithExpiration(expireAt))
}

func (ac *AstraCache) Get(key string, valueOutput interface{}) error {
	tmpData, err := ac.astraCache.Get(context.Background(), key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(tmpData.(string)), &valueOutput)
}
