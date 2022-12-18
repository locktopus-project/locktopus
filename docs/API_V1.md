# API V1 (WebSocket)

## Connection

Start a WebSocket connection to the server using the next scheme:

```
PROTO://HOST:PORT/v1?namespace=NAMESPACE&abandon-timeout-ms=TIMEOUT
```

Parameters:

- **PROTO** (required): `ws` or `wss`
- **HOST** (required): IP address or domain name
- **PORT** (required): port number
- query parameters:
  - **namespace** (required): the [namespace](CONCEPT.md#namespace)
  - **abandon-timeout-ms** (optional): the [abandon timeout](CONCEPT.md#abandon-timeout) in milliseconds

## Communication

After establishing a connection, the client is ready for starting the communication (`state` is implicitly `"ready"`).

There are two possible actions: `lock` and `release`. Each action is followed by a response from the server. A connection can operate with only one lock in a moment. After `release`, the connection can proceed to `lock` again.

### Lock

Action `lock` is available only in state `ready`.

To make a `lock`, send a JSON object with the following structure:

```typescript
type Lock = {
  action: "lock";
  resources: { type: "read" | "write"; path: string }[];
};
```

Note:

`resources.path.length` must be >= 1

Example:

```json
{
  "action": "lock",
  "resources": [
    { "type": "write", "path": ["group_x", "entity_a"] },
    { "type": "read", "path": ["group_y"] }
  ]
}
```

After the `Lock` request server responds with the client's new state. It can be either `enqueued` or `acquired`. If received `enqueued`, the client should wait until the server sends the `acquired` state.

### Release

Action `release` is available in two states: `enqueued` and `acquired`.

To perform a release, send a JSON object with the following structure:

```typescript
type Release = {
  action: "release";
};
```

Example:

```json
{
  "action": "release"
}
```

### Response from server

Each response from the server has the following structure:

```typescript
type Response = {
  id: number; // id of lock withing workspace
  action: "lock"; // action this respondes belongs to
  state: "acquired" | "enqueued" | "ready"; // new state of client
};
```

Examples:

After lock (if not acquired):

```json
{ "id": "1", "action": "lock", "state": "enqueued" }
```

After lock (if acquired):

```json
{ "id": "1", "action": "lock", "state": "acquired" }
```

After release:

```json
{ "id": "1", "action": "release", "state": "ready" }
```
