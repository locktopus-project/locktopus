# Questions And Answers

## Does the server support TLS?

No (at least, currently). The service is supposed to be used in in-house systems, not for public use. However, if you still need TLS/SSL, consider using a proxy server for that.

## Is the service persistent?

No (at least, currently). The service operates with in-memory data. Stopping an instance makes all the connections to be aborted. Currently, no restore mechanism is implemented for connections as it will rather complicate the system that is designed to be simple.
