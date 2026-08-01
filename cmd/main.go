// Package main is the CLI entrypoint.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/snobb/go-yact/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
		os.Exit(1)
	}
}

func run() error {
	var cfg config.Config

	action, err := parseArgs(&cfg)
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background())
	defer cancel()

	return action(ctx)
}
