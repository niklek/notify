// Library for sending messages to a url
package notifier

import (
	"fmt"
	"log"
	"sync"
	"time"
)

const MAX_WORKERS = 2

// Message represent a single message which will be send to a remote server
type Message struct {
	Body string // TODO: String method
}

// Notifier is the sending app
type Notifier struct {
	cfg Config
}

// Config contains all the settings for Notifier
type Config struct {
	Url        string // Url of a remote server
	NumWorkers int    // Number of workers for sending
}

// Initialize Notifier with a config
func NewNotifier(cfg Config) *Notifier {
	if cfg.Url == "" {
		log.Fatal("Url is required")
		return nil
	}
	// set defaults
	if cfg.NumWorkers == 0 {
		cfg.NumWorkers = MAX_WORKERS
	}

	return &Notifier{
		cfg: cfg,
	}
}

// Sends all messages using N workers
func (n *Notifier) Send(messages []*Message) error {
	fmt.Println("received", len(messages), "messages")

	wg := &sync.WaitGroup{}

	// sending channel
	q := make(chan Message, n.cfg.NumWorkers*2)

	// Start all workers
	for i := 0; i < n.cfg.NumWorkers; i++ {
		wg.Add(1)
		go func(i int, q chan Message) {
			_ = worker(i, q, wg)
		}(i, q)
	}

	fmt.Println("started", n.cfg.NumWorkers, "workers")

	// Distribute messages
	for _, m := range messages {
		q <- *m // TODO: wrap a Message with an error
	}
	close(q)

	// TODO: retry logic

	// Wait to complete
	fmt.Println("waiting...")
	wg.Wait()
	fmt.Println("done")

	return nil
}

// Worker: reads from a q channel and sends a message
func worker(i int, q chan Message, wg *sync.WaitGroup) error {
	defer wg.Done()

	fmt.Println("id:", i, "started")
	for m := range q {
		// TODO: sending
		time.Sleep(time.Second * 2)
		fmt.Println("id:", i, "m:", m)
	}
	fmt.Println("id:", i, "completed")
	return nil
}
