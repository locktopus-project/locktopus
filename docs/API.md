# Locktopus API

Locktopus runs an HTTP server. Currently, only one API version is supported.

Also, here are two GET endpoints for getting the server's status and statistics.

## Status page

Send a GET request to `/` (root) to get the user-readable status page of the server.

## Namespace stats

Send GET request to `/stats?namespace={NAMESPACE_NAME}` to get the `NAMESPACE_NAME`'s statistics in JSON format.
The structure of the output has not a stable interface and exposes some internal statistics. If the namespace has not been opened by clients, the server responds with 404.

## V1 (Websocket)

Refer to [API_V1](./API_V1.md) to get the details of the **V1 WebSocket** protocol.
