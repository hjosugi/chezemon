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
	"strings"
	"time"

	"github.com/hjosugi/chezemon/internal/command"
	"github.com/hjosugi/chezemon/internal/server"
	"github.com/hjosugi/chezemon/internal/state"
)

func main() {
	var (
		listenAddr = flag.String("listen", "127.0.0.1:0", "loopback address to listen on")
		openUI     = flag.Bool("open", false, "open the UI in the default browser")
		snapshot   = flag.Bool("snapshot", false, "print a fresh JSON snapshot and exit")
		timeout    = flag.Duration("timeout", 3*time.Minute, "timeout for a chezmoi operation")
		debug      = flag.Bool("debug", false, "enable debug request logs")
	)
	flag.Parse()

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
