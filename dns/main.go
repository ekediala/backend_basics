package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/signal"
)

func main() {
	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()

	const name = "dns"
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	log = log.With("app", name)
	slog.SetDefault(log)

	if len(os.Args) != 2 {
		slog.ErrorContext(ctx, "main", "error", fmt.Sprintf("expected exactly one argument; got %d", len(os.Args)-1))
		os.Exit(1)
	}

	u, err := url.Parse(os.Args[1])
	if err != nil {
		slog.ErrorContext(ctx, "main", "host", u.Host, "error", err.Error())
		os.Exit(1)
	}

	ips, err := net.LookupIP(u.Host)
	if err != nil {
		slog.ErrorContext(ctx, "main", "host", u.Host, "error", err.Error())
		os.Exit(1)
	}

	if len(ips) == 0 {
		slog.ErrorContext(ctx, "main", "error", fmt.Sprintf("no ips found for %s", u.Host))
		os.Exit(1)
	}

	for _, ip := range ips {
		if ip.To4() != nil {
			slog.InfoContext(ctx, "ipv4", "ip", ip.String())
			goto IPV6
		}
	}

IPV6:
	for _, ip := range ips {
		if ip.To4() == nil {
			slog.InfoContext(ctx, "ipv6", "ip", ip.String())
			return
		}
	}

}
