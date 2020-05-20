// Utility to send messages using notifier library
package main

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"notify/notifier"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	// TODO: Read url from a param
	url := "localhost:8080"

	// TODO: Read interval
	interval := 1

	// Buffer size to limit Parser
	messagesCap := 100
	// Messages buffer to collect from Parser
	// Parser is blocked when the channel is full
	messagesCh := make(chan notifier.Message, messagesCap)

	// Init Reader for Parser
	var in = bufio.NewScanner(os.Stdin)

	// Init WaitGroup and cancellation context
	wg := &sync.WaitGroup{}
	ctx, cancelFn := context.WithCancel(context.Background())

	// Initialize Notifier
	cfg := notifier.Config{
		Url: url,
	}
	n := notifier.NewNotifier(ctx, cfg)

	// Register signal handler, only SIGINT
	sigc := make(chan os.Signal)
	signal.Notify(sigc, syscall.SIGINT)
	// Wait for sygnal, cancel the process
	go func() {
		<-sigc
		fmt.Println("[SIGINT] signal")
		cancelFn()
	}()

	// Start Parser
	go Parser(ctx, in, messagesCh)

	// Start Sender
	//wg.Add(1)
	Sender(ctx, n, messagesCh, interval) // wg,

	// We need to wait for Sender to complete
	fmt.Println("[MAIN] waiting for Sender to complete...")
	wg.Wait()

	fmt.Println("[MAIN] is complete")
}

// Parser reads from Scanner and sends to out channel each line as Message
func Parser(ctx context.Context, in *bufio.Scanner, out chan<- notifier.Message) {
	// Completed parsing, no more new messages
	defer close(out)

	fmt.Println(time.Now(), "[PARSER] starting...")
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
		}

		// temp slow down the Parser up to 5ms per line read
		r := rand.Intn(50)
		time.Sleep(time.Duration(r) * time.Millisecond)
	}
}

// Sender reads from in channel, collects messages into a local buffer
// Every <interval> * seconds flushes the local buffer to Notifier
func Sender(ctx context.Context, n *notifier.Notifier, in <-chan notifier.Message, interval int) { // , wg *sync.WaitGroup
	//defer wg.Done()

	// Setup timer for intervals
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	// TODO: make a buffered channel
	var messages []notifier.Message // local buffer

	for {
		select {
		case m, ok := <-in:
			if !ok {
				// The in channel is closed, stop collecting messages
				fmt.Println("[SENDER] in channel is closed")
				n.Send(messages)
				// TODO: handle unsent messages
				return
			}
			// collect a message to the buffer
			messages = append(messages, m)

		case <-ticker.C:
			// Skip empty sendings
			if len(messages) == 0 {
				continue
			}

			// Send messages from the buffer and flush the buffer
			n.Send(messages)
			messages = []notifier.Message{}

		case <-ctx.Done():
			// Sending via context to the Notifier
			fmt.Println("[SENDER] received [STOP] signal")
			return
		}
	} // for
}
