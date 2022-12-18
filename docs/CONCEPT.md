# The Concept

This document describes the concept, definitions and workflow of Locktopus and in a user-friendly manner.

## **Lock**

The common definition of lock can be found on [Wikipedia](<https://en.wikipedia.org/wiki/Lock_(computer_science)>). What we want to know in context of this doc, is that the **lock** is the access to a set of **resources** (one or more).

## **Resource**

A lock resource is an abstraction of what should be locked. It is represented by **_path_** consisting of string **_segments_**. For example, a referrence to user `foo.bar@fizz.buzz` in `IT` department can be expressed with path `["user", "department/IT", "foo.bar@fizz.buzz"]`. A resource is not required to specify concrete entity, e.g. if we want to access all users in the department, we then specify the path to be `["user", "department/IT"]`. If we want to access all the users, we can use the next path: `["user"]`. When we want to lock the whole **namespace**, our single option to do that is to use path an empty path `[]`.

Notes:

- Locktopus implements tree locking: parent paths respect child path and vice-versa. E.g. locking `["A", "B"]` for write coflicts with either `["A", "B", "C"]` and `["A"]`.
- **range locks are not implemented**, e.g. using asterisk for locking `*@fizz.buzz` is not possible. The **path segment is threated as a specific string token**. However, range locks might be implemented on the application level. In this specific case, we can _group_ (logically) users by domain and use this form of path: `["user", "department/IT", "domain/fizz.buzz", "foo.bar"]` for locking a specific user, and this path `["user", "department/IT", "domain/fizz.buzz"]` for locking the whole domain `fizz.buzz`. But then be sure to use only this form, as using it together with the former one will end up in data races.
- It does not matter whether _plural_ on _singular_ form of resources is used (`user` vs `users`). It does not affect the performance either. But be sure to have the same form across your applicaiton.
- Trying to lock an empty set of resources is forbidden.
- Locking redundant resources (shadowed by other resources within the lock) is allowed. For example, if we lock all users for write and a specific one for read (or write):

  > WRITE: `["user"]` _<-- to lock all users for write_
  >
  > READ: ["user", "department/IT", "foo.bar@fizz.buzz"]` _<-- to lock a specific user for read. Actually, we dont need this, since we already lock all users for write_

  But doing that vice-versa pretty has sence and is not a redundant locking:

  > READ: `["user"]` _<-- to lock all users for read_
  >
  > WRITE: ["user", "department/IT", "foo.bar@fizz.buzz"]` _<-- to lock a specific user for write. This is shadowed by the first resource_

## Lock Type

Each resource in the lock should be specified with the lock type: **write** or **read**. Conceptually, they repeat Go's [RWMutex](https://pkg.go.dev/sync#RWMutex).

- **WRITE**: exclusive access to a resource. Having acquired the lock with with the resource, other clients cannot acquire their locks with the resource until it is released by the locker. And vice-versa, the resource cannot be acquired for write if it is not released by other clients yet.
- **READ**: shared access to a resource. Can be acquired when there are no other locks to the resource except read ones.

## Namespace

Namespaces are used to separate locks path's from each other. Working in different namespaces of a server is logically the same as working with different servers.
Specifying the namespace is required by clients. A namespace is created as soon as the first client connects to it.

## Connection Lifecycle and Workflow

The communication is stateful and synchronous. It begins with establishing a **connection** to the server by the client. The connection is reusable after the lock is released. There can be only one lock managed by connection at a moment.

A client can perform either of two commands: **LOCK** or **RELEASE** depending on in which of the three states it is: **READY**, **ENQUEUED** or **ACQUIRED**.

The states are as follows:

| State        | Description                                                                                                                                                                    |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **READY**    | The client is ready to perform locking. This is the default state of the new connection. **LOCK** is permitted                                                                 |
| **ENQUEUED** | The lock has been enqueued but not acquired because some of the **_resources_** are taken by another client. Wait until it is **ACQUIRED**. Premature **RELEASE** is permitted |
| **ACQUIRED** | The lock has been completely taken by the client. **RELEASE** is permitted                                                                                                     |

Client commands are as follows:

- **LOCK**: Try locking a set of **_resources_**. The server will immediately respond with a state indicating whether the lock has been **ENQUEUED** or **ACQUIRED**. If enqueued, the client will be then notified as soon as the lock is acquired.
- **RELEASE**: Release all the **_resources_** locked/enqueued by the precedent **LOCK** command. The server will immediately respond with the state **READY**.

Notes:

- If connection has been closed (for any reason), there is no way to restore the lock but to reconnect and lock the resources again.

- <a name="abandon-timeout"></a> Abandon Timeout. If connection has been closed without releasing the lock, the server will release it after the timeout (`abandon lock timeout`). The timeout value can be specified when making a connection, otherwise the default server value is used. The value should be as big as needed for closing access to all the resources needed to be locked plus the delay to recongnize the connection is closed on the client side.
