// Library for sending messages to a url
package notifier

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

const MAX_WORKERS = 10

// Message represent a single message which will be send to a remote server
type Message struct {
	Body string // TODO: String method
	Err  error
}

// Notifier manages sending incoming messages to a target url
type Notifier struct {
	cfg Config
	ctx context.Context
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
	}
}

// Sends all messages to url using N workers
func (n *Notifier) Send(messages []Message) {
	fmt.Println("[NOTIFIER] received", len(messages), "messages")

	wg := &sync.WaitGroup{}

	// sending channel
	q := make(chan Message, n.cfg.NumWorkers*2)

	// Start all workers
	// TODO: only once on Start
	for i := 0; i < n.cfg.NumWorkers; i++ {
		wg.Add(1)
		go worker(n.ctx, i, q, wg)
	}

	fmt.Println("started", n.cfg.NumWorkers, "workers")

	// Distribute new messages to workers
	for _, m := range messages {
		q <- m // TODO: wrap a Message with an error
	}
	close(q)

	// TODO: retry logic
	// TODO: err channel

	// Wait to complete
	fmt.Println("[NOTIFIER] waiting for workers...")
	wg.Wait()
	fmt.Println("[NOTIFIER] is complete")
}

// Worker: reads from a q channel and sends a message
func worker(ctx context.Context, i int, q chan Message, wg *sync.WaitGroup) {
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
}
