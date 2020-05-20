// Library for sending messages to a target url via POST using multiple workers
package notifier

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Default number of workers for sending messages
const MAX_WORKERS = 100

// HTTP Request timeout
const HTTP_REQUEST_TIMEOUT = 10

// TCP timeout, TSL handshake timeout
const HTTP_TRANSPORT_TIMEOUT = 5

// Message represent a single message which will be send to a remote server
type Message struct {
	Body string // TODO: String method
	Err  error
}

// Notifier manages sending incoming messages to a target url
type Notifier struct {
	cfg    Config
	ctx    context.Context
	stopFn context.CancelFunc
	wg     *sync.WaitGroup
	q      chan Message // buffered channel for sending messages, buffer size is cfg.NumWorkers * 2
}

// Config contains all the settings for Notifier
type Config struct {
	Url        string // Url of a remote server
	NumWorkers int    // Number of workers for sending
}

// Initialize Notifier with a config
func NewNotifier(cfg Config) *Notifier {
	if cfg.Url == "" {
		log.Fatal("[NOTIFIER] Url is required")
		return nil
	}
	// set defaults
	if cfg.NumWorkers == 0 {
		cfg.NumWorkers = MAX_WORKERS
	}

	// Cancellation context to stop workers
	ctx, stopFn := context.WithCancel(context.Background())

	return &Notifier{
		cfg:    cfg,
		ctx:    ctx,
		stopFn: stopFn,
		wg:     &sync.WaitGroup{},
		q:      make(chan Message, cfg.NumWorkers*2),
	}
}

// Init internal queue and Start workers
func (n *Notifier) Start() {
	// Start cfg.NumWorkers workers
	for i := 0; i < n.cfg.NumWorkers; i++ {
		n.wg.Add(1)
		go worker(n.ctx, i, n.q, n.cfg.Url, n.wg)
	}

	fmt.Println("[NOTIFIER] started", n.cfg.NumWorkers, "workers")
}

// Handle shutdown, wait for all workers to complete
func (n *Notifier) Stop() {
	// no more new messages
	close(n.q)

	// Send stop to workers
	n.stopFn()

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

	fmt.Println("[NOTIFIER] all messages are distributed")
}

// Worker: reads from a q channel and sends a message
func worker(ctx context.Context, i int, q <-chan Message, url string, wg *sync.WaitGroup) {
	defer wg.Done()
	// TODO: unsent messages can go to err channel
	// Drain channel on cancel
	defer func() {
		for range q {
		}
	}()

	var err error
	// create a HTTP client for sending all the notifications
	client := newHTTPClient()

	for m := range q {
		select {
		case <-ctx.Done():
			// The worker stops sending new messages
			fmt.Println("[WORKER", i, "] received [STOP] signal")
			return
		default:
			// Sending a notification
			err = sendNotificationWithClient(client, url, m.Body)
			if err != nil {
				// TODO: Move the message into err channel
				m.Err = err
				log.Println("[WORKER", i, "] error:", err)
				continue
			}
			fmt.Println("id:", i, "[SENT] m:", m)
		}
	}
	fmt.Println("[WORKER", i, "] completed")
}

// Sends a notification via POST to url using HTTP client
func sendNotificationWithClient(c *http.Client, url string, body string) error {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close() // we do not need body

	if resp.StatusCode != http.StatusOK {
		return errors.New("Response status code:" + strconv.Itoa(resp.StatusCode))
	}

	return nil
}

// Creates a new custom HTTP client with timeouts: HTTP_TIMEOUT
func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: time.Second * HTTP_REQUEST_TIMEOUT,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: HTTP_TRANSPORT_TIMEOUT * time.Second,
			}).Dial,
			TLSHandshakeTimeout: HTTP_TRANSPORT_TIMEOUT * time.Second,
		},
	}
}
