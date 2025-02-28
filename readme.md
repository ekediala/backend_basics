# TCP Tools

This repository contains a collection of TCP networking tools written in Go. Each tool serves a different purpose and can be used to understand or work with TCP connections in various contexts.

## Tools

### DNS

A simple DNS lookup tool that resolves domain names to IP addresses.

```
cd tcp/dns
go run main.go <URL>
```

This tool accepts a URL as an argument and performs a DNS lookup to resolve the host to both IPv4 and IPv6 addresses (if available). It outputs the results to stderr in JSON format.

### SendReq

A tool for sending HTTP requests over TCP and displaying the raw response.

```
cd tcp/sendreq
go run main.go [-method <METHOD>] [-host <HOST>] [-path <PATH>] [-port <PORT>]
```

Options:
- `-method`: HTTP method to use (default: GET)
- `-host`: Host to connect to (default: localhost)
- `-path`: Path to request (default: /)
- `-port`: Port to connect to (default: 8080)

This tool establishes a TCP connection to the specified host and port, sends an HTTP request, and prints the raw response to stdout.

### TCPUpperEcho

A TCP server that echoes back received messages in uppercase.

```
cd tcp/tcpupperecho
go run main.go [-p <PORT>]
```

Options:
- `-p`: Port to listen on (default: 8080)

The server listens for TCP connections on the specified port. When a client connects, it reads lines of text from the client, converts them to uppercase, and echoes them back.

### WriteTCP

A tool for sending raw data over TCP from stdin.

```
cd tcp/write_tcp
go run main.go [-p <PORT>]
```

Options:
- `-p`: Port to connect to (default: 8080)

This tool connects to a TCP server on localhost at the specified port. It forwards anything typed in stdin to the server and prints any responses received from the server.

## Architecture

These tools showcase various aspects of TCP networking in Go:

1. **Connection establishment**: Examples of client-side (`DialTCP`) and server-side (`ListenTCP`, `Accept`) connection management
2. **Data transmission**: Reading from and writing to TCP connections
3. **Error handling**: Proper handling of network errors
4. **Context usage**: Using context for cancellation and timeouts
5. **Concurrency**: Using goroutines to handle multiple connections or simultaneous read/write operations

## Technologies

- Go 1.23.1
- Standard library packages:
  - `net`: For TCP networking
  - `log/slog`: For structured logging
  - `bufio`: For buffered I/O
  - `context`: For cancellation and timeouts
  - `os/signal`: For handling OS signals (like Ctrl+C)

## Common Design Patterns

1. **Worker pool pattern** in tcpupperecho: Using a fixed number of worker goroutines to handle connections
2. **Signal handling**: Using `signal.NotifyContext` to handle OS signals for graceful shutdown
3. **Structured logging**: Using `log/slog` for consistent, structured logging across all tools
4. **Buffered I/O**: Using `bufio.Scanner` for efficient line-based reading from connections

## Error Handling

All tools include robust error handling, logging errors with context, and exiting with non-zero status codes when appropriate.
