package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/natun-ai/natun/internal/plugin"
	"github.com/natun-ai/natun/pkg/api"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"strconv"
	"time"
)

type state struct {
	client redis.UniversalClient
}

func (s *state) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// New returns a new redis state implementation.
func New(client redis.UniversalClient) (api.State, error) {
	// Load Lua scripts in advance. This is useful in case we have permissions issue, so we'll detect it in advance.
	err := scripts.Load(client)
	if err != nil {
		return nil, err
	}
	return &state{client}, nil
}

func init() {
	const name = "redis"
	plugin.Configurers.Register(name, BindConfig)
	plugin.StateFactories.Register(name, StateFactory)
}

func StateFactory(viper *viper.Viper) (api.State, error) {
	// Initialize redis client
	var redisTLS *tls.Config = nil
	if viper.GetBool("redis-tls") {
		redisTLS = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	addrs := viper.GetStringSlice("redis")
	if len(addrs) == 0 {
		return nil, fmt.Errorf("redis: no redis addresses provided")
	}

	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:            addrs,
		DB:               viper.GetInt("redis-db"),
		Password:         viper.GetString("redis-pass"),
		Username:         viper.GetString("redis-user"),
		SentinelUsername: viper.GetString("redis-sentinel-user"),
		SentinelPassword: viper.GetString("redis-sentinel-pass"),
		MasterName:       viper.GetString("redis-master"),
		TLSConfig:        redisTLS,
	})

	// Create a redis state management layer from the client
	return New(redisClient)
}
func BindConfig(set *pflag.FlagSet) error {
	pflag.StringArrayP("redis", "r", []string{}, "Redis URI")
	pflag.String("redis-user", "", "Redis username")
	pflag.String("redis-pass", "", "Redis password")
	pflag.String("redis-sentinel-user", "", "Redis Sentinel username")
	pflag.String("redis-sentinel-pass", "", "Redis Sentinel password")
	pflag.String("redis-master", "", "Redis Sentinel master name")
	pflag.Bool("redis-tls", false, "Enable TLS for Redis")
	pflag.Int("redis-db", 0, "Redis DB")
	return nil
}

func setTimestamp(ctx context.Context, tx redis.Cmdable, key string, ts time.Time, ttl time.Duration) *redis.Cmd {
	key = fmt.Sprintf("%s:ts", key)
	dur := int64(ttl / time.Millisecond)
	if ttl < time.Millisecond {
		dur = 1
	}
	return luaMax.Run(ctx, tx, []string{key}, ts.UnixMicro(), dur)
}
func setTimestampExpireAt(ctx context.Context, tx redis.Cmdable, key string, ts time.Time, xat time.Time) *redis.Cmd {
	return luaMax.Run(ctx, tx, []string{key}, ts.UnixMicro(), xat.UnixMilli())
}
func getTimestamp(ctx context.Context, tx redis.Cmdable, key string) (*time.Time, error) {
	s, err := tx.Get(ctx, fmt.Sprintf("%s:ts", key)).Result()
	if err != nil {
		return nil, fmt.Errorf("unable to fetch timestamp for primitiveKey %s: %w", key, err)
	}

	nts, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("unable to parse timestamp for primitiveKey %s: %w", key, err)
	}
	ts := time.UnixMicro(nts)
	return &ts, nil
}
