// Utility to send text messages using Notifier package to a server
//
// Notifier will be initialized with a url taken from flags, url is required
// Notifier.Start() creates N workers and is waiting for incoming messages from Sender process
//
// Parser reads stdin, wraps lines into a message and sends them to Sender via channel
//
// Sender takes messages from the channel, collects them into a local buffered channel
// and sends them at once on a time interval
// The interval has a default value 5 seconds, but can be specified in a flag
//
// The utility listens for SIGINT, SIGTERM
// On signal, Sender stops sending new messages, but will wait for workers to complete
//
// HandleErrors receives failed or cancelled messages from Notifier package to print basic counter
package main

import (
	"bufio"
	"context"
	"flag"
	log "github.com/sirupsen/logrus"
	"notify/notifier"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Parser's channel size, limits fast Parser
const parserqSize = 400

// Sender's channel size, limits messages which will be send to Notifier at once
const senderqSize = 200

var (
	url      string
	interval int // Two flags sharing the variable, so we can have a shorthand.
)

func init() {
	const (
		intervalDefault = 5 // Default sending interval
		intervalUsage   = "Notification interval, sec"
		urlUsage        = "Target server url for sending notifications"
	)

	flag.IntVar(&interval, "interval", intervalDefault, intervalUsage)
	flag.IntVar(&interval, "i", intervalDefault, intervalUsage+" (shorthand)") // short interval flag
	flag.StringVar(&url, "url", "", urlUsage)

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

func main() {
	flag.Parse()

	log.WithFields(log.Fields{
		"interval": interval,
		"url":      url,
	}).Info("process started")

	// Init Scanner for Parser
	var in = bufio.NewScanner(os.Stdin)

	// Cancellation context to stop the process (parsing, sending)
	ctx, cancelFn := context.WithCancel(context.Background())

	// Initialize Notifier with a confif
	cfg := notifier.Config{
		Url: url, // TODO: url validation
		//NumWorkers: 100, // Custom number of sending workers
	}
	n, err := notifier.NewNotifier(cfg)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Register signal handler, only SIGINT
	sigc := make(chan os.Signal)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	// Wait for sygnal, stop the process (parsing, reading)
	go func() {
		<-sigc
		log.Info("interrupting the process...")
		cancelFn()
	}()

	// Read channel to collect messages from Parser
	// Parser is blocked when the channel is full
	parserq := make(chan notifier.Message, parserqSize)
	// Start Parser
	go Parser(ctx, in, parserq)

	// Start error handling
	go HandleErrors(n.ErrChan())

	// Start Notifier
	n.Start()
	// Start Sender
	Sender(ctx, n, parserq, interval)

	log.Info("process complete")
}

// Parser reads from Scanner and sends to out channel each line as a notifier.Message
func Parser(ctx context.Context, in *bufio.Scanner, out chan<- notifier.Message) {
	// Completed parsing, no more new messages
	defer close(out)
	i := 0
	for in.Scan() {
		line := in.Text()
		if line == "" {
			continue
		}

		select {
		case <-ctx.Done():
			// On cancel stop reading and sending new messages
			log.Info("read interrupted")
			return
		default:
			// Create and send a message to a channel
			// Is blocked when the channel is full
			out <- notifier.Message{
				Body: line,
			}
			i++
		}
	}
	log.Infof("read and sent %d messages", i)
}

// Sender reads from in channel, collects messages into a local buffered channel
// Every <interval> * seconds flushes collected messages to Notifier
func Sender(ctx context.Context, n *notifier.Notifier, in <-chan notifier.Message, interval int) {
	// Setup timer for intervals
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()
	// Allow Notifier to complete
	defer n.Stop()

	// Init sender queue to avoid sending all at once (buffered)
	senderq := make(chan notifier.Message, senderqSize)
	defer close(senderq)

	// Read messages from Parser into the intermediate chanel
	for m := range in {
		select {
		case <-ticker.C:
			// On timer: collect messages from the queue into a slice
			messages := queueToSlice(senderq)
			messages = append(messages, m) // append missing message on this iteration
			// Send collected messages
			n.Send(messages)
		case senderq <- m:
			// the senderq is full, waiting for timer to proceed
		case <-ctx.Done():
			// Handling intermediate queue
			messages := queueToSlice(senderq)
			log.Infof("send interrupted, %d messages will not be send", len(messages))
			return
		}
	} // for range in

	// Check the queue and send the rest
	<-ticker.C
	messages := queueToSlice(senderq)
	n.Send(messages)
}

// queueToSlice reads from a buffered channel all items into a slice, then returns the slice
func queueToSlice(q <-chan notifier.Message) []notifier.Message {
	// Init a slice of messages
	messages := make([]notifier.Message, 0, senderqSize)
	for {
		select {
		case msg := <-q:
			messages = append(messages, msg)
		default:
			return messages
		}
	}
}

// Handles failed messages
// TODO: retry logic or printing into a separate error log
func HandleErrors(in <-chan notifier.Message) {
	i := 0
	for m := range in {
		i++
		log.Debugf("message:%s failed with error:%s\n", m.Body, m.Err)
	}
	log.Warnf("%d messages failed to send", i)
}
