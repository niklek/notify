// Utility to send messages using notifier library
package main

import (
	"fmt"
	"notify/notifier"
)

func main() {
	// Read url from a param
	url := "localhost:8080"
	// TODO: Read interval

	// TODO: Read messages
	messages := []*notifier.Message{
		&notifier.Message{
			Body: "test 1",
		},
		&notifier.Message{
			Body: "test 2",
		},
		&notifier.Message{
			Body: "test 3",
		},
	}

	fmt.Println("starting with", len(messages), "messages")

	cfg := notifier.Config{
		Url: url,
	}

	n := notifier.NewNotifier(cfg)

	// TODO: send messages every X seconds
	_ = n.Send(messages)

	// TODO: handle graceful shutdown
	fmt.Println("complete")
}
