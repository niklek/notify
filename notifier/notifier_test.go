package notifier

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Test basic Send using 2 messages
// Test POST method on the target server
// Test POST body (message content) on the target server
// Test no failed messages after sending
func TestSend(t *testing.T) {

	const message = "test message"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected ‘POST’ request method, got ‘%s’", r.Method)
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Error while reading request body: %s", err)
		}

		s := string(body)
		if s != message {
			t.Errorf("Expected message %s received %s", message, s)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Init Notifier with the following settings
	n, err := NewNotifier(Config{
		Url:            ts.URL, // test server url
		NumWorkers:     2,
		MsgChanSize:    2,
		MsgErrChanSize: 2,
	})
	if err != nil {
		t.Errorf("%s", err)
	}
	// Start workers
	n.Start()
	// Send message slice
	n.Send([]Message{
		Message{
			Body: message,
		},
		Message{
			Body: message,
		},
	})

	msgErrChan := n.ErrChan()
loop:
	for {
		select {
		case m := <-msgErrChan:
			t.Errorf("Unexpected message in the error channel: %s %s", m.Body, m.Err)
		case <-time.After(time.Second):
			break loop
		}
	}

	// Complete Notifier
	n.Stop()
}

// Test receiving failed messages when server is not available
// Test failed messages body and non empty error
func TestSendFails(t *testing.T) {

	const message = "test message"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	// Init Notifier with the following settings
	n, err := NewNotifier(Config{
		Url:            ts.URL, // test server url
		NumWorkers:     2,
		MsgChanSize:    2,
		MsgErrChanSize: 2,
	})
	if err != nil {
		t.Errorf("%s", err)
	}
	// Start workers
	n.Start()
	// Send message slice
	n.Send([]Message{
		Message{
			Body: message,
		},
		Message{
			Body: message,
		},
	})

	msgErrChan := n.ErrChan()
	i := 0
loop:
	for {
		select {
		case m := <-msgErrChan:
			i++
			if m.Err == nil {
				t.Errorf("Expected message with an error, received %s", m.Err)
			}
			if m.Body != message {
				t.Errorf("Expected message with body %s, received %s", message, m.Body)
			}
		case <-time.After(time.Second):
			break loop
		}
	}
	if i != 2 {
		t.Errorf("Expected number of failed messages %d received %d", 2, i)
	}

	// Complete Notifier
	n.Stop()
}
