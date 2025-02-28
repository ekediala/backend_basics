package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
)

func main() {
	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()

	const appName = "tcupperecho"
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	log = log.With("appName", appName)
	slog.SetDefault(log)

	port := flag.Int("p", 8080, "port to listen on")
	flag.Parse()

	// ListenTCP creates a TCP listener accepting connections on the given address.
	// TCPAddr represents the address of a TCP end point; it has an IP, Port, and Zone, all of which are optional.
	// Zone only matters for IPv6; we'll ignore it for now.
	// If we omit the IP, it means we are listening on all available IP addresses; if we omit the Port, it means we are listening on a random port.
	// We want to listen on a port specified by the user on the command-line.
	// see https://golang.org/pkg/net/#ListenTCP and https://golang.org/pkg/net/#Dial for details.
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: *port})
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	numWorkers := runtime.NumCPU()
	connChan := make(chan net.Conn, numWorkers)
	wg := sync.WaitGroup{}
	wg.Add(numWorkers)

	go func() {
		for range numWorkers {
			go worker(ctx, connChan, &wg)
		}
	}()

	go func() {
		<-ctx.Done()
		slog.InfoContext(ctx, "main", "message", "received shutdown signal")

		close(connChan)
		wg.Wait()

		listener.Close()
	}()

	slog.InfoContext(ctx, "main", "message", "listening for connections", "port", *port)
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				slog.InfoContext(ctx, "main", "message", "connection closed")
				return
			}
			panic(err)
		}
		connChan <- conn
	}

}

func worker(ctx context.Context, connChan <-chan net.Conn, wg *sync.WaitGroup) {
	defer wg.Done()

	for conn := range connChan {
		echoUpper(ctx, conn, conn)
		conn.Close()
	}

}

func echoUpper(ctx context.Context, w io.Writer, r io.Reader) {
	for scanner := bufio.NewScanner(r); scanner.Scan(); {
		_, err := fmt.Fprintf(w, fmt.Sprintf("%s\n", strings.ToUpper(scanner.Text())))
		if err != nil {
			slog.ErrorContext(ctx, "echoUpper", "error", err.Error())
		}

		if err := scanner.Err(); err != nil {
			slog.ErrorContext(ctx, "echoUpper", "error", err.Error())
		}
	}
}
