package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/kelseyhightower/envconfig"

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
		fmt.Println("\nCommands:")
		fmt.Println("  proxy    Start a proxy server")
		fmt.Println("  client   Start a client")
		fmt.Printf("\nPlease run '%s <cmd> -h' for more information", command)
		fmt.Println("")
	}

	var debug bool
	var configPath string

	global.BoolVar(&debug, "d", false, "debug output")
	global.StringVar(&configPath, "c", config.ConfigFileName, "config file")

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

	logger := initLogger(debug)

	switch subCommand {
	case "proxy":
		return handleProxy(logger, configPath, subCommandArgs)
	case "client":
		return handleClient(logger, configPath, subCommandArgs)
	default:
		global.Usage()
		return nil, fmt.Errorf("unknown command: %s", subCommand)
	}
}

func handleProxy(logger logger.Logger, configPath string, args []string) (Action, error) { //nolint:dupl
	fs := flag.NewFlagSet("proxy", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := proxy.Config{}

	if err := config.Load(configPath, &cfg); err != nil {
		return nil, err
	}
	cfg.SetDefaults()

	fs.StringVar(&cfg.TLS.CAPath, "ca", cfg.TLS.CAPath, "CA certificate path")
	fs.StringVar(&cfg.TLS.CertPath, "cert", cfg.TLS.CertPath, "TLS certificate path")
	fs.StringVar(&cfg.TLS.KeyPath, "key", cfg.TLS.KeyPath, "TLS key path")

	fs.StringVar(&cfg.ProxyAddr, "addr", cfg.ProxyAddr, "address to listen on")

	fs.DurationVar(&cfg.KeepAliveInterval, "i", proxy.DefaultKeepAliveInterval, "keep-alive interval")
	fs.DurationVar(&cfg.KeepAliveTimeout, "t", proxy.DefaultKeepAliveTimeout, "keep-alive timeout")

	fs.Usage = func() {
		fs.SetOutput(os.Stdout)
		defer fs.SetOutput(io.Discard)

		fmt.Printf("Usage: %s [global flags] proxy [args]\n", command)
		fs.PrintDefaults()
		fmt.Println("\nEnvironment variables can be provided:")
		_ = envconfig.Usage(config.EnvVarPrefix, &cfg)
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	logger.Debug("config", "config", cfg)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return Action(func(ctx context.Context) error {
		logger.DebugContext(ctx, "config", "config", cfg)
		return proxy.Run(ctx, logger, &cfg)
	}), nil
}

func handleClient(logger logger.Logger, configPath string, args []string) (Action, error) { //nolint:dupl
	fs := flag.NewFlagSet("client", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := client.Config{}

	if err := config.Load(configPath, &cfg); err != nil {
		return nil, err
	}

	cfg.SetDefaults()

	fs.StringVar(&cfg.TLS.CAPath, "ca", cfg.TLS.CAPath, "CA certificate path")
	fs.StringVar(&cfg.TLS.CertPath, "cert", cfg.TLS.CertPath, "TLS certificate path")
	fs.StringVar(&cfg.TLS.KeyPath, "key", cfg.TLS.KeyPath, "TLS key path")

	fs.StringVar(&cfg.ProxyAddr, "addr", cfg.ProxyAddr, "address of proxy to connect to")
	fs.StringVar(&cfg.ToAddr, "to", cfg.ToAddr, "address to tunnel connections from")
	fs.IntVar(&cfg.LocalPort, "local_port", cfg.LocalPort, "port to local connection")

	fs.DurationVar(&cfg.KeepAliveInterval, "i", client.DefaultKeepAliveInterval, "keep-alive interval")
	fs.DurationVar(&cfg.KeepAliveTimeout, "t", client.DefaultKeepAliveTimeout, "keep-alive timeout")

	fs.Usage = func() {
		fs.SetOutput(os.Stdout)
		defer fs.SetOutput(io.Discard)

		fmt.Printf("Usage: %s [global flags] client [args]\n", command)
		fs.PrintDefaults()
		fmt.Println("\nEnvironment variables can be provided:")
		_ = envconfig.Usage(config.EnvVarPrefix, &cfg)
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	return Action(func(ctx context.Context) error {
		logger.DebugContext(ctx, "config", "config", cfg)
		return client.Run(ctx, logger, &cfg)
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
