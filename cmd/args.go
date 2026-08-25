package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/kelseyhightower/envconfig"
	"github.com/spf13/pflag"

	"github.com/snobb/go-yact/internal/certgen"
	"github.com/snobb/go-yact/internal/client"
	"github.com/snobb/go-yact/internal/config"
	"github.com/snobb/go-yact/internal/logger"
	"github.com/snobb/go-yact/internal/proxy"
)

type Action func(ctx context.Context) error

var command string

func parseArgs() (Action, error) {
	command = os.Args[0]

	global := pflag.NewFlagSet("yact", pflag.ContinueOnError)

	global.Usage = func() {
		// global.SetOutput(os.Stdout)
		// defer global.SetOutput(io.Discard)

		fmt.Printf("Usage: %s [global flags] <command> [command flags]\n", command)
		fmt.Println("\nGlobal flags:")
		global.PrintDefaults()
		fmt.Println("\nCommands:")
		fmt.Println("  proxy      Start a proxy server")
		fmt.Println("  client     Start a client")
		fmt.Println("  make-certs Generate mTLS certificates")
		fmt.Printf("\nPlease run '%s <cmd> -h' for more information", command)
		fmt.Println("")
	}

	var (
		help       bool
		debug      bool
		configPath string
	)

	global.BoolVarP(&help, "help", "h", false, "show help")
	global.BoolVarP(&debug, "debug", "d", false, "debug output")
	global.StringVarP(&configPath, "config", "c", config.ConfigFileName, "config file")

	if err := global.Parse(os.Args[1:]); err != nil {
		return nil, err
	}

	remaining := global.Args()
	if len(remaining) == 0 {
		global.Usage()
		if help {
			return nil, nil
		}
		return nil, fmt.Errorf("missing command")
	}

	if help {
		// pass -h flag to subcommands if provided.
		remaining = append(remaining, "-h")
	}

	subCommand := remaining[0]
	subCommandArgs := remaining[1:]

	logger := initLogger(debug)

	switch subCommand {
	case "proxy":
		return handleProxy(logger, configPath, subCommandArgs)
	case "client":
		return handleClient(logger, configPath, subCommandArgs)
	case "make-certs":
		return handleMakeCerts(subCommandArgs)
	default:
		global.Usage()
		return nil, fmt.Errorf("unknown command: %s", subCommand)
	}
}

func handleProxy(logger logger.Logger, configPath string, args []string) (Action, error) { //nolint:dupl
	fs := pflag.NewFlagSet("proxy", pflag.ContinueOnError)

	cfg := proxy.Config{}

	if err := config.Load(configPath, &cfg); err != nil {
		return nil, err
	}
	cfg.SetDefaults()

	fs.StringVarP(&cfg.TLS.CertDir, "cert-dir", "R", cfg.TLS.CertDir, "certificate directory")
	fs.StringVarP(&cfg.ProxyAddr, "address", "a", cfg.ProxyAddr, "address to listen on")
	fs.DurationVarP(&cfg.KeepAliveInterval, "keep-alive-interval", "I", proxy.DefaultKeepAliveInterval, "keep-alive interval")
	fs.DurationVarP(&cfg.KeepAliveTimeout, "keep-alive-timeout", "T", proxy.DefaultKeepAliveTimeout, "keep-alive timeout")

	fs.Usage = func() {
		// fs.SetOutput(os.Stdout)
		// defer fs.SetOutput(io.Discard)

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
	fs := pflag.NewFlagSet("client", pflag.ContinueOnError)

	cfg := client.Config{}

	if err := config.Load(configPath, &cfg); err != nil {
		return nil, err
	}

	cfg.SetDefaults()

	fs.StringVarP(&cfg.TLS.CertDir, "cert-dir", "R", cfg.TLS.CertDir, "certificate directory")
	fs.StringVarP(&cfg.ProxyAddr, "address", "a", cfg.ProxyAddr, "address of proxy to connect to")
	fs.StringVarP(&cfg.ToAddr, "to", "t", cfg.ToAddr, "address to tunnel connections from")
	fs.IntVarP(&cfg.LocalPort, "local-port", "p", cfg.LocalPort, "port to local connection")

	fs.DurationVarP(&cfg.KeepAliveInterval, "keep-alive-interval", "I", proxy.DefaultKeepAliveInterval, "keep-alive interval")
	fs.DurationVarP(&cfg.KeepAliveTimeout, "keep-alive-timeout", "T", proxy.DefaultKeepAliveTimeout, "keep-alive timeout")

	fs.Usage = func() {
		// fs.SetOutput(os.Stdout)
		// defer fs.SetOutput(io.Discard)

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

func handleMakeCerts(args []string) (Action, error) {
	fs := pflag.NewFlagSet("make-certs", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	certDir := config.DefaultCertDir()
	fs.StringVarP(&certDir, "cert-dir", "R", certDir, "certificate directory")

	fs.Usage = func() {
		fs.SetOutput(os.Stdout)
		defer fs.SetOutput(io.Discard)

		fmt.Printf("Usage: %s make-certs [args]\n", command)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	return Action(func(ctx context.Context) error {
		return certgen.Generate(certDir)
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
