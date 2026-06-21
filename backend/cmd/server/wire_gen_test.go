package main

import (
	"testing"

	adminplussub2api "github.com/Wei-Shaw/sub2api/internal/adminplus/app/sub2api"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestProvideServiceBuildInfo(t *testing.T) {
	in := handler.BuildInfo{
		Version:   "v-test",
		BuildType: "release",
	}
	out := provideServiceBuildInfo(in)
	require.Equal(t, in.Version, out.Version)
	require.Equal(t, in.BuildType, out.BuildType)
}

func TestProvideCleanup_WithMinimalDependencies_NoPanic(t *testing.T) {
	cfg := &config.Config{}
	emailQueueSvc := service.NewEmailQueueService(nil, 1)
	billingCacheSvc := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	opsSystemLogSinkSvc := service.NewOpsSystemLogSink(nil)
	idempotencyCoordinator := service.NewIdempotencyCoordinator(nil, service.DefaultIdempotencyConfig())
	idempotencyCleanupSvc := service.NewIdempotencyCleanupService(nil, cfg)
	service.SetDefaultIdempotencyCoordinator(idempotencyCoordinator)

	cleanup := provideCleanup(
		nil, // entClient
		nil, // redis
		&service.OpsMetricsCollector{},
		&service.OpsAggregationService{},
		&service.OpsAlertEvaluatorService{},
		&service.OpsCleanupService{},
		&service.OpsScheduledReportService{},
		opsSystemLogSinkSvc,
		emailQueueSvc,
		billingCacheSvc,
		idempotencyCoordinator,
		idempotencyCleanupSvc,
		adminplussub2api.Sub2APIRedis{},
		nil,
		nil,
	)

	require.NotPanics(t, func() {
		cleanup()
	})
	require.Nil(t, service.DefaultIdempotencyCoordinator())
}
