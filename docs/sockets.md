# Networking Notes (Sockets, File Descriptors, Listening Sockets)

## Socket

* A socket is a **kernel object** that represents one endpoint of a network connection.
* It stores:

  * Local IP and port
  * Remote IP and port (if connected)
  * Send/receive buffers
  * TCP state (`LISTEN`, `ESTABLISHED`, etc.)
* Applications never manipulate the socket directly; they use a **file descriptor (FD)**.

---

## File Descriptor (FD)

* An FD is a small integer that identifies an open resource in a process.
* It is an index into the process's file descriptor table.

Example:

```text
FD Table

0 → stdin
1 → stdout
2 → stderr
3 → file.txt
4 → socket
```

* The FD itself is **not** the socket or file.
* The kernel uses the FD to locate the underlying object.

---

## Standard File Descriptors

```text
0 → stdin   (Standard Input)
1 → stdout  (Standard Output)
2 → stderr  (Standard Error)
```

* `stdin` is usually connected to the keyboard.
* `stdout` is where normal output is written.
* `stderr` is used for error messages.

Example:

```go
fmt.Println("Hello")                 // stdout (FD 1)
fmt.Fprintln(os.Stderr, "Error")     // stderr (FD 2)
```

---

## Everything is a File Descriptor

Unix treats many resources uniformly:

* Files
* Sockets
* Pipes
* Terminals
* Devices

All are accessed through file descriptors.

---

## What `net.Listen()` Does

Calling:

```go
ln, err := net.Listen("tcp", ":6379")
```

internally performs:

```text
socket()
    ↓
setsockopt()
    ↓
bind()
    ↓
listen()
```

### `socket()`

Creates a new socket object in the kernel.

### `bind()`

Associates the socket with an IP address and port.

Example:

```text
0.0.0.0:6379
```

### `listen()`

Marks the socket as a **listening socket**, allowing it to accept incoming TCP connections.

---

## Listening Socket

A listening socket:

* Waits for new connection requests.
* Does **not** exchange application data.
* Exists only to accept new clients.

Example:

```text
Listening Socket (FD 5)

        ↓

Incoming Clients

        ↓

Accept Queue
```

---

## `Accept()`

Calling:

```go
conn, err := ln.Accept()
```

causes the kernel to:

1. Remove a client from the accept queue.
2. Create a **new connected socket**.
3. Return a new file descriptor for that socket.

Example:

```text
FD 5 → Listening Socket

FD 8 → Client A

FD 9 → Client B

FD 10 → Client C
```

The listening socket remains active to accept future connections.

---

## Why Each Client Gets a New Socket

Every TCP connection maintains independent state:

* Sequence numbers
* Send buffer
* Receive buffer
* TCP state
* Remote IP and port

Therefore, each client requires its own connected socket.

---

## Client and Server on the Same Machine

Communication uses the **loopback interface (`127.0.0.1`)**.

```text
Client
   ↓
Kernel
   ↓
Loopback Interface
   ↓
Kernel
   ↓
Server
```

* No network cable or Wi-Fi is used.
* TCP handshake still occurs.
* Packets never leave the machine.

---

## Client and Server on Different Machines

Data flows through:

```text
Application
    ↓
Socket
    ↓
Kernel
    ↓
Network Card (NIC)
    ↓
Router
    ↓
Internet
    ↓
Server NIC
    ↓
Kernel
    ↓
Socket
    ↓
Application
```

The operating system handles:

* Packet creation
* Routing
* Retransmissions
* Ordering
* Congestion control
* Flow control

Applications simply call:

```go
conn.Write(data)
conn.Read(buffer)
```

---

## Relationship Between FD and Socket

```text
Process

FD Table

5
│
▼
Kernel Socket Object
```

When the application calls:

```text
write(fd)
```

the kernel:

1. Looks up the FD.
2. Finds the socket object.
3. Writes data into the socket's send buffer.

The same applies for `read(fd)`.

---

## Key Takeaways

* A socket is a kernel-managed communication endpoint.
* A file descriptor is a process-local integer that references an open resource.
* `net.Listen()` creates, binds, and starts a listening socket.
* A listening socket only accepts connections; it does not carry application data.
* Each accepted client gets its own connected socket and file descriptor.
* Whether communicating over `127.0.0.1` or the Internet, applications use the same `Read` and `Write` APIs; the operating system handles the underlying networking.
