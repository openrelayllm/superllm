package sub2api

import (
	"database/sql"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/bizlogs"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/google/wire"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

var ProviderSet = wire.NewSet(
	ProvideReadSQLDB,
	ProvideReadRedis,
	NewSQLRepository,
	NewRuntimeRepository,
	wire.Bind(new(Repository), new(*SQLRepository)),
	wire.Bind(new(RuntimeReader), new(*RuntimeRepository)),
	ProvideRoutingPort,
	ProvideService,
)

func ProvideService(repo Repository, routing Sub2APIRoutingPort, runtimeRepo RuntimeReader, recorder *bizlogs.Recorder) *Service {
	return NewService(repo, runtimeRepo).WithRoutingPort(routing).WithDiagnostics(recorder)
}

func ProvideRoutingPort(local *SQLRepository, cfg *config.Config, client *http.Client) Sub2APIRoutingPort {
	if ShouldUseRemoteAdminAPIRoutingPortFromConfig(cfg) {
		remote, err := NewRemoteAdminAPIRoutingPortFromConfig(cfg, client, local)
		if err != nil {
			return NewFailingRoutingPort(err)
		}
		return remote
	}
	return local
}

type ReadDB struct {
	DB *sql.DB
}

func ProvideReadSQLDB(defaultDB *sql.DB, cfg *config.Config) ReadDB {
	dsn := sub2APIReadonlyDatabaseURL(cfg)
	if dsn == "" {
		return ReadDB{DB: defaultDB}
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return ReadDB{DB: defaultDB}
	}
	db.SetMaxOpenConns(16)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)
	return ReadDB{DB: db}
}

type Sub2APIRedis struct {
	Client     *redis.Client
	Configured bool
	Owned      bool
}

func ProvideReadRedis(defaultRedis *redis.Client, cfg *config.Config) Sub2APIRedis {
	redisURL := sub2APIReadonlyRedisURL(cfg)
	dbOverride, hasDBOverride := sub2APIReadonlyRedisDB(cfg)
	if redisURL == "" && !hasDBOverride {
		return Sub2APIRedis{Client: defaultRedis, Configured: defaultRedis != nil}
	}

	var opts *redis.Options
	if redisURL != "" {
		parsed, err := redis.ParseURL(redisURL)
		if err != nil {
			return Sub2APIRedis{Client: defaultRedis, Configured: defaultRedis != nil}
		}
		opts = parsed
	} else {
		opts = &redis.Options{
			Addr:         cfg.Redis.Address(),
			Password:     cfg.Redis.Password,
			DB:           cfg.Redis.DB,
			DialTimeout:  time.Duration(cfg.Redis.DialTimeoutSeconds) * time.Second,
			ReadTimeout:  time.Duration(cfg.Redis.ReadTimeoutSeconds) * time.Second,
			WriteTimeout: time.Duration(cfg.Redis.WriteTimeoutSeconds) * time.Second,
			PoolSize:     cfg.Redis.PoolSize,
			MinIdleConns: cfg.Redis.MinIdleConns,
		}
	}
	if hasDBOverride {
		opts.DB = dbOverride
	}
	return Sub2APIRedis{Client: redis.NewClient(opts), Configured: true, Owned: true}
}

func sub2APIReadonlyDatabaseURL(cfg *config.Config) string {
	if cfg != nil {
		return strings.TrimSpace(cfg.Sub2API.ReadonlyDatabaseURL)
	}
	return strings.TrimSpace(os.Getenv("SUB2API_READONLY_DATABASE_URL"))
}

func sub2APIReadonlyRedisURL(cfg *config.Config) string {
	if cfg != nil {
		return strings.TrimSpace(cfg.Sub2API.ReadonlyRedisURL)
	}
	return strings.TrimSpace(os.Getenv("SUB2API_READONLY_REDIS_URL"))
}

func sub2APIReadonlyRedisDB(cfg *config.Config) (int, bool) {
	if cfg != nil {
		if cfg.Sub2API.ReadonlyRedisDB >= 0 {
			if cfg.Sub2API.ReadonlyRedisDB == 0 && strings.TrimSpace(cfg.Sub2API.ReadonlyRedisURL) == "" {
				return 0, false
			}
			return cfg.Sub2API.ReadonlyRedisDB, true
		}
		return 0, false
	}
	raw := strings.TrimSpace(os.Getenv("SUB2API_READONLY_REDIS_DB"))
	if raw == "" {
		return 0, false
	}
	db, err := strconv.Atoi(raw)
	if err != nil || db < 0 {
		return 0, false
	}
	return db, true
}
