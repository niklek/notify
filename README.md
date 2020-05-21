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

`Notify` is an utility to send text messages using `Notifier` package.

`Notifier` will be initialized with `url` taken from a flag, *url* is required.

On Start, `Notifier` will create N workers to handle the incoming messages.

`Notify` has the following parts:

- `Parser` reads from `Scanner` (stdin) and sends to a sending channel each line as a `notifier.Message`

- `Sender` reads from the sending channel, collects messages into a local buffered channel in order to send them at once to `Notifier` on time interval.
   Sending interval has default value 5 seconds, but can be specified in a flag.
   `Sender` allows `Notifier` to complete the sending on `SIGINT` or `SIGTERM`

- `HandleErrors` receives failed or cancelled messages from `Notifier`. 
   At the moment `HandleErrors` simple prints failed messages.


## Installation

```
cd cmd/notify/
go build 
```


## Start an example server
Example server helps to check

```
cd cmd/server/
go run server.go 
```


## Run an example
Basic debug messages will be printed.
```
./notify --url=http://localhost:8080/notify -i 1 < messages.txt 
```