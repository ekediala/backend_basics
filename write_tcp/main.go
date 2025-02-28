package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
)

func main() {
	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()

	const name = "writetcp"
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	log = log.With("app", name)
	slog.SetDefault(log)

	port := flag.Int("p", 8080, "port to connect to")
	flag.Parse()

	conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{Port: *port})
	if err != nil {
		slog.ErrorContext(ctx, "main", "error", fmt.Sprintf("error connecting to localhost:%d: %v", *port, err))
		os.Exit(1)
	}
	defer conn.Close()

	slog.InfoContext(ctx, "main", "info",fmt.Sprintf("connected to %s: will forward stdin", conn.RemoteAddr()))

	// spawn a goroutine to read incoming lines from the server and print them to stdout.
	// TCP is full-duplex, so we can read and write at the same time; we just need to spawn a goroutine to do the reading.
	go func() {
		for connScanner := bufio.NewScanner(conn); connScanner.Scan(); {
			slog.InfoContext(ctx, "connScanner", "server message", connScanner.Text())
			if err := connScanner.Err(); err != nil {
				slog.ErrorContext(ctx, "connScanner", "error", fmt.Sprintf("error reading from %s: %v", conn.RemoteAddr(), err))
				os.Exit(1)
			}
		}
	}()

	go func() {
		<-ctx.Done()
		slog.InfoContext(ctx, "main", "info", "shutdown signal received.")
		conn.Close()
		os.Exit(0)
	}()

	for stdInScanner := bufio.NewScanner(os.Stdin); stdInScanner.Scan(); {
		slog.InfoContext(ctx, "stdInScanner", "info", fmt.Sprintf("sent: %s", stdInScanner.Text()))
		if _, err := conn.Write(fmt.Appendf(stdInScanner.Bytes(), "\n")); err != nil {
			slog.ErrorContext(ctx, "stdInScanner", "error", fmt.Sprintf("error writing to %s: %v", conn.RemoteAddr(), err))
		}

		if err := stdInScanner.Err(); err != nil {
			slog.ErrorContext(ctx, "stdInScanner", "error", fmt.Sprintf("error reading from stdin: %v", err))
			os.Exit(1)
		}
	}

}
