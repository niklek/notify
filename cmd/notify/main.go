// Utility to send messages using notifier library
//
// Notifier will be initialized with url taken from a flag, url is required
// on Start Notifier will create N workers to handle the incoming messages
//
// Parser reads from Scanner (stdin) and sends to a sending channel each line as a notifier.Message
//
// Sender reads from the sending channel, collects messages into a local buffered channel in order to send them at once to Notifier on time interval
// Sending interval has default value 5 seconds, but can be specified in a flag
// Sender allows Notifier to complete the sending on SIGINT/SIGTERM
//
// HandleErrors receives failed or cancelled messages from Notifier
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"notify/notifier"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Default sending interval
const intervalDefault = 5

// Parser's channel size, limits fast Parser
const parserqSize = 400

// Sender's channel size, limits messages which will be send to Notifier at once
const senderqSize = 200

// Workaround to parse short and long flags
// TODO: parse interval when not a number
var intervalFlag = flag.Int("interval", intervalDefault, "Notification interval, sec") // long interval flag
func init() {
	flag.IntVar(intervalFlag, "i", intervalDefault, "Notification interval, sec") // short interval flag
}

func main() {
	var (
		url      string
		interval int
	)

	// Parse flags
	// TODO: print usage, support --help
	flag.StringVar(&url, "url", "", "Target server url for sending notifications")
	flag.Parse()
	interval = *intervalFlag

	// Init Scanner for Parser
	var in = bufio.NewScanner(os.Stdin)

	// Cancellation context to stop the process (parsing, sending)
	ctx, cancelFn := context.WithCancel(context.Background())

	// Initialize Notifier with a confif
	cfg := notifier.Config{
		Url: url,
		//NumWorkers: 100, // Custom number of sending workers
	}
	n := notifier.NewNotifier(cfg)

	// Register signal handler, only SIGINT
	sigc := make(chan os.Signal)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	// Wait for sygnal, stop the process (parsing, reading)
	go func() {
		<-sigc
		fmt.Println("[SIG*] signal from OS")
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

	fmt.Println("[MAIN] is complete")
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
			fmt.Println("[PARSER] received [STOP] signal")
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
	fmt.Println("[PARSER] sent", i, "messages")
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
			fmt.Println("[SENDER] received [STOP] signal.", len(messages), "messages will not be send")
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
// TODO: retry logic
func HandleErrors(in <-chan notifier.Message) {
	for m := range in {
		if m.Err != nil {
			fmt.Println("[HandleErrors] failed message:", m.Body, "error:", m.Err)
		} else {
			fmt.Println("[HandleErrors] cancelled message:", m.Body)
		}
	}
}
