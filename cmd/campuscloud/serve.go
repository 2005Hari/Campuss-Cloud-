package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/2005Hari/campuscloud/internal/api"
)

// newServeCmd starts the HTTP API a web dashboard talks to (PRD section
// 17: Web-based admin dashboard). It runs on the same host as Docker —
// the API shells out to `docker`/`docker compose` exactly like every
// other campuscloud command — while the dashboard itself can be deployed
// anywhere (e.g. Vercel) and call this API over HTTPS.
func newServeCmd(a *app) *cobra.Command {
	var addr string
	var token string
	var origins string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the HTTP API used by the CampusCloud web dashboard",
		Long: `serve starts a token-authenticated JSON API exposing status, health,
containers, logs, backups, and deploy/start/stop/restart/backup/restore
actions. Point a web dashboard's API URL at this server (behind HTTPS —
put a reverse proxy such as Caddy or nginx in front of it in production).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if token == "" {
				token = os.Getenv("CAMPUSCLOUD_API_TOKEN")
			}
			if token == "" {
				return fmt.Errorf("no API token: pass --token or set CAMPUSCLOUD_API_TOKEN")
			}

			var allowedOrigins []string
			for _, o := range strings.Split(origins, ",") {
				if o = strings.TrimSpace(o); o != "" {
					allowedOrigins = append(allowedOrigins, o)
				}
			}
			if len(allowedOrigins) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: no --cors-origin set; browser-based dashboards on another origin will be blocked")
			}

			server, err := api.NewServer(a.cfg, a.client, api.Options{Token: token, AllowedOrigins: allowedOrigins})
			if err != nil {
				return err
			}

			httpServer := &http.Server{
				Addr:              addr,
				Handler:           server.Handler(),
				ReadHeaderTimeout: 10 * time.Second,
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			errCh := make(chan error, 1)
			go func() { errCh <- httpServer.ListenAndServe() }()

			fmt.Fprintf(cmd.OutOrStdout(), "campuscloud API listening on %s\n", addr)

			select {
			case err := <-errCh:
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					return err
				}
				return nil
			case <-ctx.Done():
				fmt.Fprintln(cmd.OutOrStdout(), "shutting down...")
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				return httpServer.Shutdown(shutdownCtx)
			}
		},
	}

	cmd.Flags().StringVar(&addr, "addr", ":9090", "address to listen on")
	cmd.Flags().StringVar(&token, "token", "", "API bearer token (or set CAMPUSCLOUD_API_TOKEN)")
	cmd.Flags().StringVar(&origins, "cors-origin", "", "comma-separated list of allowed browser origins (e.g. your Vercel URL)")
	return cmd
}
