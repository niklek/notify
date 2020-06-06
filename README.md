# notifier

`notifier` is a package for sending text messages to a target url via HTTP `POST` requests.

Exported methods:
- `NewNotifier` init method, creates `Notifier` with a config.
- `Start` starts N workers, by default 20. Each worker creates a custom HTTP client with specified timeouts, used for sending messages.
- `Send` receives messages as `[]Message`, adds them to the sending channel which will be read by workers.
- `Stop` waits for the workers to complete the recent sending. Has to be called at the end.
- `ErrChan` returns an error channel to handle failed messages.

## Logging

The package uses `github.com/sirupsen/logrus` logger, logs as JSON to *stdout*.
Log level can be set via `LOG_LEVEL` env variable, default is debug level.
Logging has to be handled externally, see [12factor.net/logs](https://12factor.net/logs).

## Run tests

```
cd notifier/
go test -v
```

# notify cli tool

`notify` is an utility to send text messages to a target server using `notifier` package.
Accepts the following arguments:
- *url*, must be set via `--url` parameter, required
- *interval*, can be set via `--interval` or `-i`, has a default value 5 seconds

## What it does internally

`Parser` reads *stdin*, wraps lines into a *message* and sends them to `Sender` via a channel.
`Sender` takes messages from the channel, collects them into a local buffer and sends them at once on a time interval.
`HandleErrors` receives failed messages from `notifier` package for further handling.

## Graseful shutdown

`notify` listens for *SIGINT*, *SIGTERM*
On signal, `Sender` will stop sending new messages and wait for `notifier` to complete.

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

Basic debug messages will be printed in *stdout*.
```
./notify --url=http://localhost:8080/notify -i 1 < messages.txt 
```