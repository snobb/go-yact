package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
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

	// disable interspersed flags to let sub-flagsets get options from pflag.
	// NOTE: the flags must not clash between root level and sub-flagsets.
	pflag.CommandLine.SetInterspersed(false)

	pflag.Usage = func() {
		fmt.Printf("Usage: %s [pflag flags] <command> [command flags]\n", command)
		fmt.Println("\npflag flags:")
		pflag.PrintDefaults()
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

	pflag.BoolVarP(&help, "help", "h", false, "show help")
	pflag.BoolVarP(&debug, "debug", "d", false, "debug output")
	pflag.StringVarP(&configPath, "config", "c", config.ConfigFileName, "config file")

	pflag.Parse()

	remaining := pflag.Args()
	if len(remaining) == 0 {
		pflag.Usage()
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
		pflag.Usage()
		return nil, fmt.Errorf("unknown command: %s", subCommand)
	}
}

func handleProxy(logger logger.Logger, configPath string, args []string) (Action, error) { //nolint:dupl
	fs := pflag.NewFlagSet("proxy", pflag.ContinueOnError)

	cfg := proxy.Config{}
	cfg.SetDefaults()

	if err := config.Load(configPath, &cfg); err != nil {
		return nil, err
	}

	fs.StringVarP(&cfg.TLS.CertDir, "cert-dir", "R", cfg.TLS.CertDir, "certificate directory")
	fs.StringVarP(&cfg.ProxyAddr, "address", "a", cfg.ProxyAddr, "address to listen on")
	fs.DurationVarP(&cfg.KeepAliveInterval, "keep-alive-interval", "I", proxy.DefaultKeepAliveInterval, "keep-alive interval")
	fs.DurationVarP(&cfg.KeepAliveTimeout, "keep-alive-timeout", "T", proxy.DefaultKeepAliveTimeout, "keep-alive timeout")

	fs.Usage = func() {
		fmt.Printf("Usage: %s [pflag flags] proxy [args]\n", command)
		fs.PrintDefaults()
		fmt.Println("\nEnvironment variables can be provided:")
		_ = envconfig.Usage(config.EnvVarPrefix, &cfg)
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return Action(func(ctx context.Context) error {
		logger.DebugContext(ctx, " starting proxy with config", "config", cfg)
		return proxy.Run(ctx, logger, &cfg)
	}), nil
}

func handleClient(logger logger.Logger, configPath string, args []string) (Action, error) { //nolint:dupl
	fs := pflag.NewFlagSet("client", pflag.ContinueOnError)

	cfg := client.Config{}
	cfg.SetDefaults()

	if err := config.Load(configPath, &cfg); err != nil {
		return nil, err
	}

	var bindAddrs []string

	fs.StringVarP(&cfg.TLS.CertDir, "cert-dir", "R", cfg.TLS.CertDir, "certificate directory")
	fs.StringVarP(&cfg.ProxyAddr, "address", "a", cfg.ProxyAddr, "address of proxy to connect to")
	fs.StringArrayVarP(&bindAddrs, "listen", "L", []string{}, "bind_host:bind_port:local:port to listen on. Eg. 127.0.0.1:8080:8080 or :8080:8080")
	fs.DurationVarP(&cfg.KeepAliveInterval, "keep-alive-interval", "I", proxy.DefaultKeepAliveInterval, "keep-alive interval")
	fs.DurationVarP(&cfg.KeepAliveTimeout, "keep-alive-timeout", "T", proxy.DefaultKeepAliveTimeout, "keep-alive timeout")

	fs.Usage = func() {
		fmt.Printf("Usage: %s [pflag flags] client [args]\n", command)
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

	if len(addrs) > 0 {
		cfg.ToAddrs = addrs
	}

	return Action(func(ctx context.Context) error {
		logger.DebugContext(ctx, "starting client with config", "config", cfg)
		return client.Run(ctx, logger, &cfg)
	}), nil
}

func handleMakeCerts(args []string) (Action, error) {
	fs := pflag.NewFlagSet("make-certs", pflag.ContinueOnError)

	var (
		certDir   = config.DefaultCertDir()
		domains   []string
		IPAddrStr []string
		nClients  int
	)

	fs.StringVarP(&certDir, "cert-dir", "R", certDir, "certificate directory")
	fs.StringArrayVarP(&domains, "domain", "d", []string{"localhost"},
		"domain names (can be specified multiple times)")
	fs.StringArrayVarP(&IPAddrStr, "ip-address", "a", nil,
		"IP address (can be specified multiple times)")
	fs.IntVarP(&nClients, "clients", "n", 1, "number of client certificates to generate")

	fs.Usage = func() {
		fmt.Printf("Usage: %s make-certs [args]\n", command)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	ips, err := parseIPs(IPAddrStr)
	if err != nil {
		return nil, err
	}

	fmt.Println("Generating server and client certificates in", certDir)
	fmt.Println("  Domains:", strings.Join(domains, ", "))
	fmt.Println("  IP addresses:", strings.Join(IPAddrStr, ", "))
	fmt.Println("  Number of clients:", nClients)

	return Action(func(ctx context.Context) error {
		return certgen.Generate(certDir, domains, ips, nClients)
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
		return nil, nil
	}

	bindAddrs := make([]client.BindAddress, 0, len(addrs))

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

func parseIPs(ips []string) ([]net.IP, error) {
	if len(ips) == 0 {
		return nil, nil
	}

	var parsedIPs []net.IP
	for _, ip := range ips {
		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			return nil, fmt.Errorf("invalid IP address: %s", ip)
		}
		parsedIPs = append(parsedIPs, parsedIP)
	}
	return parsedIPs, nil
}
