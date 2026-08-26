package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

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

	var bindAddrs []string

	fs.StringVarP(&cfg.TLS.CertDir, "cert-dir", "R", cfg.TLS.CertDir, "certificate directory")
	fs.StringVarP(&cfg.ProxyAddr, "address", "a", cfg.ProxyAddr, "address of proxy to connect to")
	fs.StringArrayVarP(&bindAddrs, "listen", "L", []string{}, "bind_host:bind_port:local:port to listen on. Eg. 127.0.0.1:8080:8080 or :8080:8080")
	fs.DurationVarP(&cfg.KeepAliveInterval, "keep-alive-interval", "I", proxy.DefaultKeepAliveInterval, "keep-alive interval")
	fs.DurationVarP(&cfg.KeepAliveTimeout, "keep-alive-timeout", "T", proxy.DefaultKeepAliveTimeout, "keep-alive timeout")

	fs.Usage = func() {
		fmt.Printf("Usage: %s [global flags] client [args]\n", command)
		fs.PrintDefaults()
		fmt.Println("\nEnvironment variables can be provided:")
		_ = envconfig.Usage(config.EnvVarPrefix, &cfg)
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	addrs, err := parseBindAddresses(bindAddrs)
	if err != nil {
		return nil, err
	}
	cfg.ToAddrs = addrs

	return Action(func(ctx context.Context) error {
		logger.DebugContext(ctx, "config", "config", cfg)
		return client.Run(ctx, logger, &cfg)
	}), nil
}

func handleMakeCerts(args []string) (Action, error) {
	fs := pflag.NewFlagSet("make-certs", pflag.ContinueOnError)

	certDir := config.DefaultCertDir()
	fs.StringVarP(&certDir, "cert-dir", "R", certDir, "certificate directory")

	fs.Usage = func() {
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

func parseBindAddresses(addrs []string) ([]client.BindAddress, error) {
	if len(addrs) == 0 {
		return []client.BindAddress{
			{BindAddress: ":8443", LocalPort: 8443},
		}, nil
	}

	var bindAddrs []client.BindAddress

	for _, addr := range addrs {
		bindAddr, err := parseBindAddress(addr)
		if err != nil {
			return nil, err
		}
		bindAddrs = append(bindAddrs, bindAddr)
	}

	return bindAddrs, nil
}

func parseBindAddress(bindAddr string) (client.BindAddress, error) {
	bindAddrParts := strings.Split(bindAddr, ":")
	if len(bindAddrParts) != 3 {
		return client.BindAddress{}, fmt.Errorf("invalid bind address: %s", bindAddr)
	}

	localPort, err := strconv.Atoi(bindAddrParts[2])
	if err != nil {
		return client.BindAddress{}, err
	}

	return client.BindAddress{
		BindAddress: bindAddrParts[0] + ":" + bindAddrParts[1],
		LocalPort:   localPort,
	}, nil
}
