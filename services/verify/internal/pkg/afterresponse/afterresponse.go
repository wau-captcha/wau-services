// Package afterresponse runs work once the HTTP response has been written, so the caller never
// waits for it, e.g. the database writes behind /challenge/new and /challenge/verify.
package afterresponse

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// taskTimeout bounds each task, which outlives the request it was queued by.
const taskTimeout = 10 * time.Second

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
		go func() {
			for _, task := range tasks {
				runTask(ctx, task)
			}
		}()
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
	go runTask(context.WithoutCancel(ctx), task)
}

func runTask(ctx context.Context, task func(context.Context)) {
	ctx, cancel := context.WithTimeout(ctx, taskTimeout)
	defer cancel()
	task(ctx)
}
