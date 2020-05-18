// Utility to send messages using notifier library
package main

import (
	"bufio"
	"fmt"
	"notify/notifier"
	"os"
)

func main() {
	// Read url from a param
	url := "localhost:8080"
	// TODO: Read interval

	// initialize Notifier
	cfg := notifier.Config{
		Url: url,
	}
	n := notifier.NewNotifier(cfg)

	// TODO: Read messages
	var messages []*notifier.Message
	var in = bufio.NewScanner(os.Stdin)

	for in.Scan() {
		line := in.Text()
		if len(line) == 0 {
			continue
		}

		m := &notifier.Message{
			Body: line,
		}
		messages = append(messages, m)

		if len(messages) > 10 {
			fmt.Println("sending", len(messages), "messages")
			_ = n.Send(messages)
			messages = []*notifier.Message{} // reset the slice TODO: use memory pool
		}
	}

	// TODO: send messages every X seconds
	if len(messages) > 0 {
		_ = n.Send(messages)
	}

	// TODO: handle graceful shutdown
	fmt.Println("complete")
}
