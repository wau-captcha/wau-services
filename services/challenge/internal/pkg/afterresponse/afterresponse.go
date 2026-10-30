// Package afterresponse runs work once the HTTP response has been written, so the caller never
// waits for it, e.g. the database writes behind /challenge/new and /challenge/verify. Wait lets
// the service finish that work before it exits.
package afterresponse

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// taskTimeout bounds each task, which outlives the request it was queued by. It leaves a write
// time to wait for a paused database to resume, which takes 30s or more after a day asleep.
const taskTimeout = 45 * time.Second

// running tracks the background goroutines that run tasks, for Wait.
var running sync.WaitGroup

type queueKey struct{}

type queue struct {
	mu    sync.Mutex
	tasks []func(context.Context)
}

// Filter is a net/http middleware. Tasks queued with Run while the handler serves the request
// start, in the background, once the handler has written the response.
func Filter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pending := &queue{}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), queueKey{}, pending)))
		pending.mu.Lock()
		tasks := pending.tasks
		pending.tasks = nil
		pending.mu.Unlock()
		if len(tasks) == 0 {
			return
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		ctx := context.WithoutCancel(r.Context())
		// Started before this handler returns, so a server shutting down waits for the handler
		// and Wait then sees the tasks.
		running.Go(func() {
			for _, task := range tasks {
				runTask(ctx, task)
			}
		})
	})
}

// Run queues task to run after the response of the HTTP request behind ctx. Outside such a
// request, e.g. in a gRPC call, it starts the task in the background at once.
func Run(ctx context.Context, task func(context.Context)) {
	if pending, ok := ctx.Value(queueKey{}).(*queue); ok {
		pending.mu.Lock()
		pending.tasks = append(pending.tasks, task)
		pending.mu.Unlock()
		return
	}
	ctx = context.WithoutCancel(ctx)
	running.Go(func() { runTask(ctx, task) })
}

// Wait blocks until every task started so far has finished, or until ctx is done. Call it once
// the servers have stopped, so no request is left to start another, and before closing what the
// tasks write to. Each task gives up after taskTimeout, so Wait never lasts much longer.
func Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		running.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func runTask(ctx context.Context, task func(context.Context)) {
	ctx, cancel := context.WithTimeout(ctx, taskTimeout)
	defer cancel()
	task(ctx)
}
