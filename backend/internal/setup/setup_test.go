package setup

import (
	"os"
	"strings"
	"testing"
)

func TestWriteConfigFileKeepsDefaultUserConcurrency(t *testing.T) {
	t.Setenv("RUN_MODE", "simple")
	t.Setenv("DATA_DIR", t.TempDir())

	if err := writeConfigFile(&SetupConfig{}); err != nil {
		t.Fatalf("writeConfigFile() error = %v", err)
	}

	data, err := os.ReadFile(GetConfigFilePath())
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if !strings.Contains(string(data), "user_concurrency: 5") {
		t.Fatalf("config missing default user concurrency, got:\n%s", string(data))
	}
}

func TestWriteConfigFileIncludesSub2APIIntegration(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	redisDB := 1

	if err := writeConfigFile(&SetupConfig{
		Sub2API: Sub2APIConfig{
			ReadonlyDatabaseURL:  "postgresql://readonly:secret@db:5432/sub2api?sslmode=disable",
			ReadonlyRedisURL:     "redis://redis:6379/1",
			ReadonlyRedisDB:      &redisDB,
			AdminBaseURL:         "https://sub2api.example",
			AdminAPIKey:          "admin-secret",
			AllowEmbeddedGateway: true,
		},
	}); err != nil {
		t.Fatalf("writeConfigFile() error = %v", err)
	}

	data, err := os.ReadFile(GetConfigFilePath())
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"sub2api:",
		"readonly_database_url: postgresql://readonly:secret@db:5432/sub2api?sslmode=disable",
		"readonly_redis_url: redis://redis:6379/1",
		"readonly_redis_db: 1",
		"admin_plus:",
		"sub2api_admin_base_url: https://sub2api.example",
		"sub2api_admin_api_key: admin-secret",
		"allow_embedded_sub2api_gateway: true",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("config missing %q, got:\n%s", want, content)
		}
	}
}

func TestBuildDatabaseConnectionDSNsUsesPostgresForBootstrap(t *testing.T) {
	cfg := &DatabaseConfig{
		Host:     "db",
		Port:     39931,
		User:     "sub2api",
		Password: "secret",
		DBName:   "sub2api",
		SSLMode:  "disable",
	}

	bootstrapDSN, targetDSN := buildDatabaseConnectionDSNs(cfg)

	if !strings.Contains(bootstrapDSN, "dbname=postgres") {
		t.Fatalf("bootstrap DSN = %q, want default postgres database", bootstrapDSN)
	}
	if strings.Contains(bootstrapDSN, "dbname=sub2api") {
		t.Fatalf("bootstrap DSN = %q, should not connect to target database before checking/creating it", bootstrapDSN)
	}
	if !strings.Contains(targetDSN, "dbname=sub2api") {
		t.Fatalf("target DSN = %q, want configured database", targetDSN)
	}
	if strings.Contains(targetDSN, "host=db:39931") {
		t.Fatalf("target DSN = %q, lib/pq keyword DSN should keep host and port separate", targetDSN)
	}
	if !strings.Contains(targetDSN, "host=db") || !strings.Contains(targetDSN, "port=39931") {
		t.Fatalf("target DSN = %q, want separate host and non-default port", targetDSN)
	}
}

func TestBuildPostgresDSNOmitsEmptyPassword(t *testing.T) {
	cfg := &DatabaseConfig{
		Host:    "127.0.0.1",
		Port:    5432,
		User:    "root",
		DBName:  "superllm",
		SSLMode: "disable",
	}

	_, targetDSN := buildDatabaseConnectionDSNs(cfg)

	if strings.Contains(targetDSN, "password=") {
		t.Fatalf("target DSN = %q, empty password should be omitted", targetDSN)
	}
	if !strings.Contains(targetDSN, "dbname=superllm") {
		t.Fatalf("target DSN = %q, want configured database", targetDSN)
	}
}
