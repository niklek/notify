# Notifier (the Library)

Notifier is a package for sending text messages to a target url via HTTP `POST` requests.
Notifier has the following public methods:

- `Start` creates N workers, by default 20.

- `Send` receives a slice of `Message`, adds each item to the sending queue which is read by workers.

- `Stop` waits for the workers to complete the sending process. Has to be called at the end.

`Notifier` has an error queue which should be read from outside using `ErrChan` method.
Remaining failed messages in the error queue will be removed before exit when `Stop` is called.

Each worker creates a custom HTTP client with specified timeouts, used for sending messages.
Failed messages will be added to the error queue.

## Run tests

```
cd notifier/
go test -v
```

# Notify (the Executable)

`Notify` is an utility to send text messages using `Notifier` package to a server.

`Notifier` will be initialized with a `url` taken from flags, `url` is required.
`Notifier.Start()` creates N workers and is waiting for incoming messages from a `Sender` process.

`Parser` reads *stdin*, wraps lines into a *message* and sends them to `Sender` via channel.
`Sender` takes messages from the channel, collects them into a local buffered channel and sends them at once on a time interval.
The interval has a default value 5 seconds, but can be specified in a flag.

`Notify` listens for *SIGINT*, *SIGTERM*
On signal, `Sender` stops sending new messages, but will wait for workers to complete.

`HandleErrors` receives failed or cancelled messages from `Notifier` package to print basic info.

## Installation

```
cd cmd/notify/
go build 
```


## Start a server
An example server that helps to demonstrate receiving messages from `notify`.

```
cd internal/server/
go run server.go 
```


## Run an example
Basic debug messages will be printed in stdout.
```
./notify --url=http://localhost:8080/notify -i 1 < messages.txt 
```