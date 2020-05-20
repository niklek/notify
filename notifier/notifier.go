// Library for sending messages to a target url via POST using multiple workers
package notifier

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

const MAX_WORKERS = 100

// Message represent a single message which will be send to a remote server
type Message struct {
	Body string // TODO: String method
	Err  error
}

// Notifier manages sending incoming messages to a target url
type Notifier struct {
	cfg Config
	ctx context.Context
	wg  *sync.WaitGroup
	q   chan Message // internal channel for sending messages
}

// Config contains all the settings for Notifier
type Config struct {
	Url        string // Url of a remote server
	NumWorkers int    // Number of workers for sending
}

// Initialize Notifier with a config and context
func NewNotifier(ctx context.Context, cfg Config) *Notifier {
	if cfg.Url == "" {
		log.Fatal("[NOTIFIER] Url is required")
		return nil
	}
	// set defaults
	if cfg.NumWorkers == 0 {
		cfg.NumWorkers = MAX_WORKERS
	}

	return &Notifier{
		cfg: cfg,
		ctx: ctx,
		wg:  &sync.WaitGroup{},
	}
}

// Init internal queue and Start workers
func (n *Notifier) Start() {
	// Init queue for workers
	n.q = make(chan Message, n.cfg.NumWorkers*2)

	// Start cfg.NumWorkers workers
	for i := 0; i < n.cfg.NumWorkers; i++ {
		n.wg.Add(1)
		go worker(n.ctx, i, n.q, n.wg)
	}

	fmt.Println("[NOTIFIER] started", n.cfg.NumWorkers, "workers")
}

// Handle shutdown, wait for all workers to complete
func (n *Notifier) Stop() {
	close(n.q)
	// TODO: send cancel to workers in case ...
	fmt.Println("[NOTIFIER] [STOP] waiting for all workers")
	n.wg.Wait()
	fmt.Println("[NOTIFIER] is complete")
}

// Sends all messages to url using N workers
func (n *Notifier) Send(messages []Message) {
	fmt.Println("[NOTIFIER] received", len(messages), "messages")

	// Distribute new messages to workers
	for _, m := range messages {
		// Is Blocked when the buffer is full
		n.q <- m
	}

	// TODO: retry logic
	// TODO: err channel

	// Wait to complete
	//fmt.Println("[NOTIFIER] waiting for workers...")
	//n.wg.Wait()
	fmt.Println("[NOTIFIER] all messages are distributed")
}

// Worker: reads from a q channel and sends a message
func worker(ctx context.Context, i int, q <-chan Message, wg *sync.WaitGroup) {
	defer wg.Done()
	// TODO: unsent messages can go to err channel
	// Drain channel on cancel
	defer func() {
		for range q {
		}
	}()

	for m := range q {
		select {
		case <-ctx.Done():
			// The worker stops sending new messages
			fmt.Println("[WORKER", i, "] received [STOP] signal")
			return
		default:
			// TODO: sending
			time.Sleep(time.Second * 1)
			fmt.Println("id:", i, "m:", m)
		}
	}
	fmt.Println("[WORKER", i, "] completed")
}
