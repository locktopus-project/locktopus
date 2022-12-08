# LOCKTOPUS

A service for managing locks. 

## What problem does it solve?

In a distributed system there *always* is a need to coordinate access to resources. Without such a coordination, multiple processes may access the same resources at the same time (data race). This may lead to deadlocks, lost updates and consistency violations. **Locktopus** is a service that addresses this problem by serializing clients' access to conflicting resources. It receives a set of resources that need to be accessed from a client and locks them as soon as nobody alse is using them.

## Features

- **WebSocket communication** ensures a client is notified of his lock's state as soon as it is changed
- Near-**constant** lock/unlock **time**. There are no queues under the hood, just goroutines, hashmaps and mutexes
- **Read/Write** lock types for separate resources
- FIFO lock order

## Compilation

```bash
go build ./cmd/server
```

## Running

```bash
./server
```

Use `--help` (`-h`) flag to see all available options:

```
  -h, --help                     Show help message and exit
  -H, --host=                    Hostname for listening. Overrides env var LOCKTOPUS_HOST. Default: 0.0.0.0
  -p, --port=                    Port to listen on. Overrides env var LOCKTOPUS_PORT. Default: 9009
      --log-clients=             Log client sessions (true/false). Overrides env var LOCKTOPUS_LOG_CLIENTS. Default: false
      --log-locks=               Log locks caused by client sessions (true/false). Overrides env var LOCKTOPUS_LOG_LOCKS. Default: false
      --stats-interval=          Log usage statistics every N>0 seconds. Overrides env var LOCKTOPUS_STATS_INTERVAL. Default: 0 (never)
      --default-abandon-timeout= Default abandon timeout (ms) used for releasing closed connections not released by clients. Overrides env var LOCKTOPUS_DEFAULT_ABANDON_TIMEOUT. Default: 60000
```

```bash
./server
```

## Testing

Testing cab be done with old-fashioned 

```bash
go test ./...
```

**Unit tests** of the core library can be run with command

```bash
go test -timeout 30s ./pkg/... -count 10000
```

Adjust `-count` (and `-timeout` correspondingly) to increase the probability of race conditions to happen.

**E2E tests** (cmd/server) run a server instance under the hood. If you want to run a separate one, use env var `SERVER_ADDRESS`.

There is also a program which searches for bugs in the lock engine automatically. Configure constants in `./cmd/bug_finder.go` (optionally) and run it:

```bash
sh ./scripts/bug_finder.sh
```

After starting a simulation, neither stdout nor stderr outputs are expected. The simulation will stop on the first bug found, otherwise never.

## Contribution

Feel free to open issues for any reason or contact the maintainer directly.
 
## License

The software is publiched under MIT [LICENCE](./LICENCE)