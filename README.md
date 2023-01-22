# LOCKTOPUS

A service for managing locks.

Visit [locktopus.xyz](https://locktopus.xyz) to get more detailed info.

This repository contains the following:

- Locktopus server for standalone usage
- lock engine library for integration into other projects
- Locktopus Go client

## Brief description

Quite often in a backend application, multiple processes access the same data at the same time. This is called a race condition, and it is just a natural state of the things in the World. But in software, they may lead to deadlocks, lost updates, consistency violations, etc. **Locktopus** addresses this problem by serializing access to resources.

### Features

- **FIFO** lock order
- **subtree locking**. Lock resources as precise as you need
- **multiple resources** can be locked at once
- **Read/Write** locks for separate resources within one lock session
- **Live communication** (WebSocket) ensures a client is notified of his lock's state as soon as it is changed
- Near-**constant** lock/unlock **time**. There are no explicit queues under the hood, just goroutines, hashmaps and mutexes

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

## Testing

To test everything at once, run

```bash
go test ./...
```

**Unit tests** of the core library can be run with the command

```bash
go test -timeout 30s ./pkg/... -count 10000
```

Adjust `-count` (and `-timeout` correspondingly) to increase the probability of race conditions happening.

**E2E tests** (cmd/server) run a server instance under the hood. If you want to run a separate one, use env var `SERVER_ADDRESS`.

**Bug finder** is a program that searches for bugs in the lock engine automatically. Configure constants in `./cmd/bug_finder.go` (optionally) and run it:

```bash
sh ./scripts/bug_finder.sh
```

After starting a simulation, neither stdout nor stderr outputs are expected. The simulation will stop on the first bug found, otherwise never.

## Contribution

Feel free to open issues for any reason or contact the maintainer directly.

## License

The software is published under MIT [LICENCE](./LICENCE)
