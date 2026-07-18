package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/hjosugi/chezemon/internal/command"
	"github.com/hjosugi/chezemon/internal/server"
	"github.com/hjosugi/chezemon/internal/state"
)

func main() {
	var (
		listenAddr  = flag.String("listen", "127.0.0.1:0", "loopback address to listen on")
		openUI      = flag.Bool("open", false, "open the UI in the default browser")
		snapshot    = flag.Bool("snapshot", false, "print a fresh JSON snapshot and exit")
		timeout     = flag.Duration("timeout", 3*time.Minute, "timeout for a chezmoi operation")
		debug       = flag.Bool("debug", false, "enable debug request logs")
		showVersion = flag.Bool("version", false, "print the Chezemon version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println(buildVersion())
		return
	}

	if err := requireLoopback(*listenAddr); err != nil {
		fmt.Fprintln(os.Stderr, "chezemon:", err)
		os.Exit(2)
	}
	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	service := state.NewService(command.ExecRunner{}, *timeout)

	if *snapshot {
		current, err := service.Snapshot(context.Background(), true)
		if err != nil {
			logger.Error("snapshot failed", "error", err)
			os.Exit(1)
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(current); err != nil {
			logger.Error("snapshot encoding failed", "error", err)
			os.Exit(1)
		}
		return
	}

	listener, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		logger.Error("listen failed", "error", err)
		os.Exit(1)
	}

	httpServer := &http.Server{
		Handler:           server.New(service, logger),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	url := "http://" + listener.Addr().String()
	fmt.Println("Chezemon is ready:", url)
	fmt.Println("Read-only mode: no dotfiles will be changed.")

	if *openUI {
		if err := openBrowser(url); err != nil {
			logger.Warn("could not open browser", "error", err)
		}
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.Serve(listener)
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer stop()
	select {
	case <-signalCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown failed", "error", err)
		}
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}
}

// buildVersion reports what this binary actually is, from the information the
// Go toolchain stamps in at build time.
//
// The commit is included because the module version alone can mislead: a
// binary built before its release tag existed carries a pseudo-version, so
// "which build is this" is only reliably answered by the revision.
func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "chezemon (version unknown)"
	}

	var revision string
	var modified bool
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	short := revision
	if len(short) > 12 {
		short = short[:12]
	}

	// A build made from a tagged commit carries that tag. A build made from
	// any other commit carries a pseudo-version, which embeds the revision and
	// so is both redundant here and not a release — report it as "devel" so
	// the two are never confused.
	version := info.Main.Version
	if version == "" || version == "(devel)" || (short != "" && strings.Contains(version, short)) {
		version = "devel"
	}

	report := "chezemon " + version
	if short != "" {
		report += " (" + short
		if modified {
			report += ", dirty"
		}
		report += ")"
	}
	return report + " " + runtime.GOOS + "/" + runtime.GOARCH
}

func requireLoopback(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("listen address must be loopback-only (127.0.0.1, ::1, or localhost)")
	}
	return nil
}

func openBrowser(url string) error {
	var command string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		command, args = "open", []string{url}
	case "windows":
		command, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		command, args = "xdg-open", []string{url}
	}
	return exec.Command(command, args...).Start()
}
