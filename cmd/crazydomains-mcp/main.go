// Command crazydomains-mcp is a read-only MCP server for a Crazy Domains
// (Dreamscape) reseller account. Configuration is via environment variables;
// see README.md.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/goodtune/crazydomains-mcp/internal/api"
	"github.com/goodtune/crazydomains-mcp/internal/config"
	"github.com/goodtune/crazydomains-mcp/internal/httpauth"
	"github.com/goodtune/crazydomains-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	// Logs go to stderr: stdout carries the stdio transport.
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	if cfg.WritesRequested {
		logger.Warn("DREAMSCAPE_ENABLE_WRITES=true is ignored: this build has no write tools")
	}

	client, err := api.New(cfg.BaseURL, cfg.APIKey)
	if err != nil {
		return err
	}
	server := tools.NewServer(&tools.Deps{Client: client}, version)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("starting", "version", version, "env", cfg.Env, "base_url", cfg.BaseURL, "transport", cfg.Transport)
	switch cfg.Transport {
	case config.TransportHTTP:
		return serveHTTP(ctx, logger, cfg, server)
	default:
		return server.Run(ctx, &mcp.StdioTransport{})
	}
}

// newHTTPHandler returns the HTTP handler for the streamable transport: the
// MCP endpoint at /mcp behind bearer authentication, and an unauthenticated
// /healthz that reveals nothing.
func newHTTPHandler(server *mcp.Server, token string) http.Handler {
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	mux := http.NewServeMux()
	mux.Handle("/mcp", httpauth.RequireBearer(token, mcpHandler))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok\n"))
	})
	return mux
}

func serveHTTP(ctx context.Context, logger *slog.Logger, cfg *config.Config, server *mcp.Server) error {
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           newHTTPHandler(server, cfg.BearerToken),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", cfg.HTTPAddr, "path", "/mcp")
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
