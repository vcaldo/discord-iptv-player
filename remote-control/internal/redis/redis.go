package redis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
)

type Client struct {
	rdb    *redis.Client
	config *config.Config
	nrApp  *newrelic.Application
}

func NewClient(ctx context.Context, cfg *config.Config, nrApp *newrelic.Application) (*Client, error) {
	txn := nrApp.StartTransaction("redis:initialize-client")
	defer txn.End()

	ctx = newrelic.NewContext(ctx, txn)

	txn.AddAttribute("redis_address", cfg.RedisAddress)
	txn.AddAttribute("redis_db", cfg.RedisDB)

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddress,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	segment := txn.StartSegment("redis:ping")
	pong, err := rdb.Ping().Result()
	if err != nil {
		txn.NoticeError(err)
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}
	segment.End()

	log.Printf("successfully connected to Redis: %s", pong)

	return &Client{
		rdb:    rdb,
		config: cfg,
		nrApp:  nrApp,
	}, nil
}

func (c *Client) instrumentOperation(operationName string, fn func() error) error {
	txn := c.nrApp.StartTransaction("redis:" + operationName)
	defer txn.End()

	err := fn()
	if err != nil {
		txn.NoticeError(err)
	}

	return err
}

func (c *Client) StoreChannelState(guildID, channelName string) error {
	return c.instrumentOperation("store-channel-state", func() error {
		key := fmt.Sprintf("guild:%s:channel", guildID)
		return c.rdb.Set(key, channelName, 24*time.Hour).Err()
	})
}

func (c *Client) Close() error {
	return c.rdb.Close()
}
