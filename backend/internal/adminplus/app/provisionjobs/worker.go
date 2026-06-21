package provisionjobs

import (
	"context"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Worker struct {
	service     *Service
	eventWaiter RedisEventWaiter
	stop        chan struct{}
	done        chan struct{}
	once        sync.Once
	started     bool
	workerID    string
}

func NewWorker(service *Service) *Worker {
	worker := &Worker{
		service:  service,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		workerID: defaultWorkerID(),
	}
	if service != nil {
		if waiter, ok := service.publisher.(RedisEventWaiter); ok {
			worker.eventWaiter = waiter
		}
	}
	return worker
}

func ProvideWorker(service *Service) *Worker {
	worker := NewWorker(service)
	if workerEnabled() {
		worker.Start(workerInterval())
	}
	return worker
}

func (w *Worker) Start(interval time.Duration) {
	if w == nil || w.service == nil || w.started {
		return
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	w.started = true
	go func() {
		defer close(w.done)
		runCtx, cancelRun := context.WithCancel(context.Background())
		defer cancelRun()
		go func() {
			select {
			case <-w.stop:
				cancelRun()
			case <-runCtx.Done():
			}
		}()
		w.runTick()
		for {
			if w.eventWaiter != nil {
				w.waitForRedisEvent(runCtx, interval)
			} else {
				if !w.waitForInterval(runCtx, interval) {
					return
				}
			}
			select {
			case <-runCtx.Done():
				return
			default:
				w.runTick()
			}
		}
	}()
}

func (w *Worker) Stop() {
	if w == nil {
		return
	}
	w.once.Do(func() {
		if !w.started {
			return
		}
		close(w.stop)
		<-w.done
	})
}

func (w *Worker) runTick() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := w.service.PublishPendingOutbox(ctx, 100); err != nil {
		log.Printf("[AdminPlusProvisionWorker] publish outbox failed: %v", err)
	}
	for i := 0; i < 5; i++ {
		job, err := w.service.RunOnce(ctx, w.workerID)
		if err != nil {
			log.Printf("[AdminPlusProvisionWorker] run job failed: %v", err)
			return
		}
		if job == nil {
			return
		}
	}
}

func (w *Worker) waitForRedisEvent(ctx context.Context, interval time.Duration) {
	if w == nil || w.eventWaiter == nil {
		return
	}
	waitCtx, cancel := context.WithTimeout(ctx, interval)
	defer cancel()
	if _, err := w.eventWaiter.WaitProvisionEvents(waitCtx, w.workerID, interval); err != nil && !isContextDone(err) {
		log.Printf("[AdminPlusProvisionWorker] wait redis stream failed: %v", err)
	}
}

func (w *Worker) waitForInterval(ctx context.Context, interval time.Duration) bool {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func isContextDone(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func workerEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_PLUS_PROVISION_WORKER_ENABLED")))
	return value == "" || value == "1" || value == "true" || value == "yes"
}

func workerInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv("ADMIN_PLUS_PROVISION_WORKER_INTERVAL_SECONDS"))
	if raw == "" {
		return 5 * time.Second
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return 5 * time.Second
	}
	return time.Duration(seconds) * time.Second
}
