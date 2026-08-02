// Package main is the CLI entrypoint.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
		os.Exit(1)
	}
}

func run() error {
	action, err := parseArgs()
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background())
	defer cancel()

	return action(ctx)
}
