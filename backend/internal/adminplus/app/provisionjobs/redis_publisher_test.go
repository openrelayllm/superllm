package provisionjobs

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestStreamPublisherPublishesAndWaitsProvisionEvents(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	publisher := NewStreamPublisher(rdb)
	err := publisher.PublishProvisionEvent(context.Background(), OutboxEvent{
		EventID:       "event-1",
		EventType:     "supplier.key.provision.requested",
		AggregateType: "supplier_provision_job",
		AggregateID:   42,
		Payload: map[string]any{
			"job_id":      int64(42),
			"supplier_id": int64(7),
		},
	})
	require.NoError(t, err)

	count, err := publisher.WaitProvisionEvents(context.Background(), "worker-test", 50*time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	pending, err := rdb.XPending(context.Background(), provisionStreamKey, provisionStreamGroup).Result()
	require.NoError(t, err)
	require.Equal(t, int64(0), pending.Count)
}
