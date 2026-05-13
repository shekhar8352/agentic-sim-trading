package redisconn

import (
	"log"
	"os"

	"github.com/redis/go-redis/v9"
)

// New returns a Redis client from REDIS_URL, or nil if unset (local HTTP-only dev).
func New() *redis.Client {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		return nil
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		log.Printf("redis: invalid REDIS_URL: %v", err)
		return nil
	}
	return redis.NewClient(opt)
}
