// Sends messages to a server via POST using multiple workers
//
// Start creates N workers
// Send receives []Message and each message to a sending channel read by the workers
// A worker on start creates a custom HTTP client with timeouts, used for sending messages
// Stop waits for workers to complete the sending
//
// Failed messages will be forwarded to an error channel and must be read by the caller before Stop call
package notifier

import (
	"bytes"
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// Default number of workers for sending messages
const numWorkersDefault = 20

// HTTP Request timeout
const httpRequestTimeout = 10

// TCP timeout
const httpTransportTimeout = 5

// TSL handshake timeout
const httpTLSTimeout = 5

// Message represent a single message which will be send to a remote server
type Message struct {
	Body string // TODO: String method
	Err  error
}

// Notifier manages sending incoming messages to a target url
type Notifier struct {
	url        string
	numWorkers int
	ctx        context.Context
	stopFn     context.CancelFunc
	wg         *sync.WaitGroup
	msgChan    chan Message // buffered channel for sending messages
	msgErrChan chan Message // buffered channel for failed messages
}

// Config contains all the settings for Notifier
type Config struct {
	Url            string // Url of a remote server
	NumWorkers     int    // Number of workers for sending
	MsgChanSize    int    // Messages channel size
	MsgErrChanSize int    // Error channel size
}

func init() {
	// Take log level from env variable or default
	s, _ := os.LookupEnv("LOG_LEVEL")
	logLevel, err := log.ParseLevel(s)
	if err != nil {
		logLevel = log.DebugLevel
	}
	// Logger settings
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(logLevel)
}

// Initialize Notifier with a config
func NewNotifier(cfg Config) (*Notifier, error) {
	if cfg.Url == "" {
		return nil, fmt.Errorf("url is required")
	}
	// set defaults
	if cfg.NumWorkers == 0 {
		cfg.NumWorkers = numWorkersDefault
	}
	if cfg.MsgChanSize == 0 {
		cfg.MsgChanSize = cfg.NumWorkers * 5
	}
	if cfg.MsgErrChanSize == 0 {
		cfg.MsgErrChanSize = cfg.NumWorkers * 10
	}

	// Cancellation context to stop workers
	ctx, stopFn := context.WithCancel(context.Background())

	return &Notifier{
		url:        cfg.Url,
		numWorkers: cfg.NumWorkers,
		ctx:        ctx,
		stopFn:     stopFn,
		wg:         &sync.WaitGroup{},
		msgChan:    make(chan Message, cfg.MsgChanSize),
		msgErrChan: make(chan Message, cfg.MsgErrChanSize),
	}, nil
}

// Start runs workers
func (n *Notifier) Start() {
	for i := 0; i < n.numWorkers; i++ {
		n.wg.Add(1)
		go worker(n.ctx, i, n.msgChan, n.msgErrChan, n.url, n.wg)
	}

	log.Info("started", n.numWorkers, "workers")
}

// Handle shutdown, wait for all workers to complete
func (n *Notifier) Stop() {
	// Drain error channel on cancel
	defer func() {
		log.Warning("drop", len(n.msgErrChan), "messages from err channel")
		for range n.msgErrChan {
		}
	}()

	// no more new messages
	close(n.msgChan)

	// Send stop to workers
	// n.stopFn() // Disabled: allow to complete all messages

	log.Debug("waiting for workers to complete")
	n.wg.Wait()

	// no more new errors
	close(n.msgErrChan)

	log.Info("sending is complete")
}

// Send adds messages to a channel read by workers
func (n *Notifier) Send(messages []Message) {
	log.Info("received", len(messages), "messages")

	for _, m := range messages {
		// Is Blocked when the channel is full
		n.msgChan <- m
	}

	log.Debug("all messages were added to the sending channel")
}

// ErrChan returns a buffered channel to handle failed messages by the caller
func (n *Notifier) ErrChan() <-chan Message {
	return n.msgErrChan
}

// worker: reads messages from msgChan and sends via HTTP POST
// Failed messages will be added to msgErrChan
func worker(ctx context.Context, i int, msgChan <-chan Message, msgErrChan chan<- Message, url string, wg *sync.WaitGroup) {
	defer wg.Done()

	var err error
	client := newHTTPClient()

	for m := range msgChan {
		select {
		case <-ctx.Done():
			// The worker stops sending new messages, adds current message to err channel and exits
			log.Debug("worker:", i, "is interrupted")

			select {
			case msgErrChan <- m:
				log.Debug("worker:", i, "added current message to err channel before exit")
			default:
				log.Error("worker:", i, "err channel is full, could not add current message before exit")
			}
			return

		default:
			// Sending a message
			err = sendMessageWithClient(client, url, m.Body)
			if err != nil {
				// Set the error and move the message into error channel
				m.Err = err
				// Is Blocked when the error channel is full
				// TODO: can be ignored on block
				msgErrChan <- m
				continue
			}
		}
	}
	log.Info("worker:", i, "completed")
}

// Sends a message via POST to url using HTTP client
func sendMessageWithClient(c *http.Client, url string, body string) error {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close() // we do not need response body

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request has failed with status code %d", resp.StatusCode)
	}

	return nil
}

// Creates a new custom HTTP client with timeouts: HTTP_TIMEOUT
func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: time.Second * httpRequestTimeout,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: httpTransportTimeout * time.Second,
			}).Dial,
			TLSHandshakeTimeout: httpTLSTimeout * time.Second,
		},
	}
}
