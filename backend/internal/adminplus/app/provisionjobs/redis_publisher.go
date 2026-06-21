package provisionjobs

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const provisionStreamKey = "admin_plus:provision:events"
const provisionStreamGroup = "admin_plus_provision_workers"

type StreamPublisher struct {
	rdb              *redis.Client
	consumerGroupMu  sync.Mutex
	consumerGroupSet bool
}

func NewStreamPublisher(rdb *redis.Client) *StreamPublisher {
	return &StreamPublisher{rdb: rdb}
}

func (p *StreamPublisher) PublishProvisionEvent(ctx context.Context, event OutboxEvent) error {
	if p == nil || p.rdb == nil {
		return nil
	}
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}
	return p.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: provisionStreamKey,
		Values: map[string]any{
			"event_id":       event.EventID,
			"event_type":     event.EventType,
			"aggregate_type": event.AggregateType,
			"aggregate_id":   strconv.FormatInt(event.AggregateID, 10),
			"payload":        string(payload),
		},
	}).Err()
}

func (p *StreamPublisher) WaitProvisionEvents(ctx context.Context, consumerID string, block time.Duration) (int, error) {
	if p == nil || p.rdb == nil {
		return 0, nil
	}
	if err := p.ensureConsumerGroup(ctx); err != nil {
		return 0, err
	}
	if strings.TrimSpace(consumerID) == "" {
		consumerID = defaultWorkerID()
	}
	if block <= 0 {
		block = 5 * time.Second
	}
	streams, err := p.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    provisionStreamGroup,
		Consumer: consumerID,
		Streams:  []string{provisionStreamKey, ">"},
		Count:    10,
		Block:    block,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		if strings.Contains(strings.ToUpper(err.Error()), "NOGROUP") {
			p.resetConsumerGroup()
		}
		return 0, err
	}

	count := 0
	for _, stream := range streams {
		ids := make([]string, 0, len(stream.Messages))
		for _, message := range stream.Messages {
			if strings.TrimSpace(message.ID) == "" {
				continue
			}
			ids = append(ids, message.ID)
		}
		if len(ids) == 0 {
			continue
		}
		count += len(ids)
		_ = p.rdb.XAck(ctx, provisionStreamKey, provisionStreamGroup, ids...).Err()
	}
	return count, nil
}

func (p *StreamPublisher) ensureConsumerGroup(ctx context.Context) error {
	p.consumerGroupMu.Lock()
	defer p.consumerGroupMu.Unlock()
	if p.consumerGroupSet {
		return nil
	}
	err := p.rdb.XGroupCreateMkStream(ctx, provisionStreamKey, provisionStreamGroup, "0").Err()
	if err != nil && !strings.Contains(strings.ToUpper(err.Error()), "BUSYGROUP") {
		return err
	}
	p.consumerGroupSet = true
	return nil
}

func (p *StreamPublisher) resetConsumerGroup() {
	p.consumerGroupMu.Lock()
	defer p.consumerGroupMu.Unlock()
	p.consumerGroupSet = false
}
