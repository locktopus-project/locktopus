# LOCKTOPUS

A service for managing locks.

Available docs:

- [CONCEPT](./docs/CONCEPT.md)
- [API](./docs/API.md)
- [QnA](./docs/QNA.md)

## What problem does it solve?

In a distributed system there _always_ is a need to coordinate access to resources. Without such, multiple processes may access the same resources at the same time (data race). This may lead to deadlocks, lost updates and consistency violations. **Locktopus** is a service that addresses this problem by serializing clients' access to conflicting resources. It receives a set of resources that need to be accessed from a client and locks them as soon as nobody is using them.

## Features

- **FIFO** lock order
- **subtree locking**. Lock resources as precise as you need
- **multiple resources** can be locked at once
- **Read/Write** locks for separate resources within one lock session
- **Live communication** (WebSocket) ensures a client is notified of his lock's state as soon as it is changed
- Near-**constant** lock/unlock **time**. There are no explicit queues under the hood, just goroutines, hashmaps and mutexes

## Locktopus vs Redlock

One might ask why not use [Redlock](https://redis.io/docs/manual/patterns/distributed-locks/) instead. Redlock is a good tool for distributed locking, designed for running in a cluster. It is quite supported by the community and has a lot of client libraries. But there are limitations to it, and some of them are:

- no option to lock multiple resources at once
- no read/write locks, just write locks
- time overhead
- no FIFO lock order
- livelocks are possible

The brief considerations about cluster mode are given in the section below.

## Cluster

Since the service is about coordinating access to resources between other services, one might consider running it in a cluster, similar to Redlock, to avoid having a single point of failure. Here is not an in-built solution for that, but it can be implemented manually. The only thing one should remember to avoid deadlocks is to perform locking (no matter whether enqueue or acquire) on different nodes in the same order by all clients. This way all the liveness and safety properties of Redlocks are preserved and additionally, the locks will be acquired in FIFO order. The drawback here, in comparison to Redlock, is that in the optimistic case (no lock conflicts) Redlock will be faster due to parallel locking, though not providing FIFO lock order.

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

Testing can be done with old-fashioned

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
