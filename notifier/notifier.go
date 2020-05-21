// Library for sending messages to a target url via POST using multiple workers
// Start creates N workers
// Send receives a slice of Message adds each message to the sending queue used by workers
// Stop waits for workers to complete the sending from the sending queue,
// remaining failed messages in the error queue will be removed before exit
//
// Each worker creates a custom HTTP client with specified timeouts, used for sending messages
// Failed messages will be added to Notifier.qerr buffered channel
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
	q      chan Message // buffered channel for sending messages
	qerr   chan Message // buffered channel for failed messages
}

// Config contains all the settings for Notifier
type Config struct {
	Url              string // Url of a remote server
	NumWorkers       int    // Number of workers for sending
	SendingQueueSize int    // Sending queue size
	ErrorQueueSize   int    // Error queue size
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
	if cfg.SendingQueueSize == 0 {
		cfg.SendingQueueSize = cfg.NumWorkers * 3
	}
	if cfg.ErrorQueueSize == 0 {
		cfg.ErrorQueueSize = cfg.NumWorkers * 3
	}

	// Cancellation context to stop workers
	ctx, stopFn := context.WithCancel(context.Background())

	return &Notifier{
		cfg:    cfg,
		ctx:    ctx,
		stopFn: stopFn,
		wg:     &sync.WaitGroup{},
		q:      make(chan Message, cfg.SendingQueueSize),
		qerr:   make(chan Message, cfg.ErrorQueueSize),
	}
}

// Init internal queue and Start workers
func (n *Notifier) Start() {
	// Start cfg.NumWorkers workers
	for i := 0; i < n.cfg.NumWorkers; i++ {
		n.wg.Add(1)
		go worker(n.ctx, i, n.q, n.qerr, n.cfg.Url, n.wg)
	}

	fmt.Println("[NOTIFIER] started", n.cfg.NumWorkers, "workers")
}

// Handle shutdown, wait for all workers to complete
func (n *Notifier) Stop() {
	// Drain error channel on cancel
	defer func() {
		fmt.Println("[NOTIFIER] drop", len(n.qerr), "messages")
		for range n.qerr {
		}
	}()

	// no more new messages
	close(n.q)

	// Send stop to workers
	// n.stopFn() // Disabled: allow to complete all messages

	// waiting for all workers to complete
	fmt.Println("[NOTIFIER] [STOP] waiting for all workers")
	n.wg.Wait()

	// no more new errors
	close(n.qerr)

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

	fmt.Println("[NOTIFIER] all messages sent to workers")
}

// ErrChan returns a buffered channel on which the caller can receive failed messages
func (n *Notifier) ErrChan() <-chan Message {
	return n.qerr
}

// Worker: reads from a q channel and sends a message
// Failed messages with anattached errors go to qerr channel
func worker(ctx context.Context, i int, q <-chan Message, qerr chan<- Message, url string, wg *sync.WaitGroup) {
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
			// Return the last message to the error channel
			// Drop the last message when error channel is full
			select {
			case qerr <- m:
				fmt.Println("[WORKER", i, "] added last message to err channel")
			default:
				fmt.Println("[WORKER", i, "] can not add last message to err channel")
			}
			return

		default:
			// Sending a notification
			err = sendNotificationWithClient(client, url, m.Body)
			if err != nil {
				// Set the error and move the message into error channel
				m.Err = err
				// Is Blocked when the error channel is full
				// TODO: we can drop the failed message on block
				qerr <- m
				continue
			}
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
