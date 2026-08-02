package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/snobb/go-yact/internal/client"
	"github.com/snobb/go-yact/internal/config"
	"github.com/snobb/go-yact/internal/logger"
	"github.com/snobb/go-yact/internal/proxy"
)

type Action func(ctx context.Context) error

var command string

func parseArgs() (Action, error) {
	command = os.Args[0]

	global := flag.NewFlagSet("yact", flag.ContinueOnError)
	global.SetOutput(io.Discard)

	global.Usage = func() {
		global.SetOutput(os.Stdout)
		defer global.SetOutput(io.Discard)

		fmt.Printf("Usage: %s [global flags] <command> [command flags]\n", command)
		fmt.Println("\nGlobal flags:")
		global.PrintDefaults()
		fmt.Println("\nEnvironment variables can be provided:")
		fmt.Println("  YACT_CA_PATH - path to the CA certificate")
		fmt.Println("  YACT_CERT_PATH - path to the TSL certificate")
		fmt.Println("  YACT_KEY_PATH - path to the TLS certificate key")
		fmt.Println("\nCommands:")
		fmt.Println("  proxy    Start a proxy server")
		fmt.Println("  client   Start a client")
		fmt.Printf("\nPlease run '%s <cmd> -h' for more information", command)
		fmt.Println("")
	}

	commonCfg := &config.CommonConfig{}

	global.BoolVar(&commonCfg.Debug, "d", false, "debug output")
	global.StringVar(&commonCfg.CAPath, "ca", "", "CA certificate path")
	global.StringVar(&commonCfg.CertPath, "cert", "", "TLS certificate path")
	global.StringVar(&commonCfg.KeyPath, "key", "", "TLS key path")

	err := global.Parse(os.Args[1:])
	if err != nil {
		return nil, err
	}

	remaining := global.Args()
	if len(remaining) == 0 {
		global.Usage()
		return nil, fmt.Errorf("missing command")
	}

	subCommand := remaining[0]
	subCommandArgs := remaining[1:]

	logger := initLogger(commonCfg.Debug)

	switch subCommand {
	case "proxy":
		return handleProxy(commonCfg, logger, subCommandArgs)
	case "client":
		return handleClient(commonCfg, logger, subCommandArgs)
	default:
		global.Usage()
		return nil, fmt.Errorf("unknown command: %s", subCommand)
	}
}

func handleProxy(commonCfg *config.CommonConfig, logger logger.Logger, args []string) (Action, error) { //nolint:dupl
	fs := flag.NewFlagSet("proxy", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := &client.Config{
		CommonConfig: commonCfg,
	}

	fs.StringVar(&cfg.ProxyAddr, "addr", ":8008", "address to listen on")

	fs.Usage = func() {
		fs.SetOutput(os.Stdout)
		defer fs.SetOutput(io.Discard)

		fmt.Printf("Usage: %s [global flags] proxy [args]\n", command)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	cfg.ReadEnv()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return Action(func(ctx context.Context) error {
		return proxy.Run(ctx, logger, cfg.ProxyAddr)
	}), nil
}

func handleClient(commonCfg *config.CommonConfig, logger logger.Logger, args []string) (Action, error) { //nolint:dupl
	fs := flag.NewFlagSet("client", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := &proxy.Config{
		CommonConfig: commonCfg,
	}

	fs.StringVar(&cfg.ProxyAddr, "addr", ":8008", "address of proxy to listen to")
	fs.DurationVar(&cfg.KeepAliveInterval, "i", proxy.DefaultKeepAliveInterval, "keep-alive interval")
	fs.DurationVar(&cfg.KeepAliveTimeout, "t", proxy.DefaultKeepAliveTimeout, "keep-alive timeout")

	fs.Usage = func() {
		fs.SetOutput(os.Stdout)
		defer fs.SetOutput(io.Discard)

		fmt.Printf("Usage: %s [global flags] client [args]\n", command)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	cfg.ReadEnv()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return Action(func(ctx context.Context) error {
		return client.Run(ctx, logger, cfg.ProxyAddr)
	}), nil
}

func initLogger(debug bool) logger.Logger {
	if debug {
		return logger.New(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	}
	return logger.New(os.Stdout, nil)
}
