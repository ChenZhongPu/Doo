package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"doo/internal/api"
	"doo/internal/secrets"
	"doo/internal/store"
	"doo/web"
)

var (
	version   = "0.1.0"
	buildTime = "dev"
	gitCommit = ""
)

func main() {
	var (
		addrFlag    = flag.String("addr", "127.0.0.1:8787", "HTTP server listen address")
		dataFlag    = flag.String("data", "~/.doo", "Data directory path for database and keys")
		openFlag    = flag.Bool("open", false, "Open browser after server starts")
		versionFlag = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	if *versionFlag {
		fmt.Printf("Doo %s (commit %s, built %s)\n", version, gitCommit, buildTime)
		os.Exit(0)
	}

	dataDir := expandPath(*dataFlag)
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Fatalf("failed to create data directory %s: %v", dataDir, err)
	}

	keyPath := filepath.Join(dataDir, "master.key")
	keyStore, err := secrets.NewKeyStore(keyPath)
	if err != nil {
		log.Fatalf("failed to initialize key store: %v", err)
	}

	dbPath := filepath.Join(dataDir, "doo.db")
	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("failed to initialize database store: %v", err)
	}
	defer st.Close()

	warnIfNotLoopback(*addrFlag)

	secCfg := api.SecurityConfig{BindAddr: *addrFlag}
	apiServer := api.NewServer(st, keyStore, version, buildTime, gitCommit, secCfg)

	distFS, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		log.Fatalf("failed to open embedded web dist: %v", err)
	}

	fileServer := http.FileServer(http.FS(distFS))

	topMux := http.NewServeMux()
	// API routes
	topMux.Handle("/api/", apiServer.Handler())

	// Static routes
	topMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" || path == "index.html" {
			w.Header().Set("Cache-Control", "no-cache")
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if file exists in distFS
		f, err := distFS.Open(path)
		if err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for SPA routes
		w.Header().Set("Cache-Control", "no-cache")
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})

	srv := &http.Server{
		Addr:    *addrFlag,
		Handler: api.SecurityMiddleware(secCfg, topMux),
	}

	go func() {
		log.Printf("Doo server listening on http://%s", *addrFlag)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	if *openFlag {
		go func() {
			time.Sleep(100 * time.Millisecond)
			openBrowser(fmt.Sprintf("http://%s", *addrFlag))
		}()
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Doo server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
	log.Println("Doo server stopped cleanly")
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func warnIfNotLoopback(addr string) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}

	if host != "127.0.0.1" && host != "localhost" && host != "::1" && host != "[::1]" {
		log.Printf("⚠️  WARNING: doo is listening on non-loopback address %s. Host and Origin verification are enforced.", addr)
	}
}

func openBrowser(url string) {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default: // linux, bsd, etc.
		cmd = "xdg-open"
		args = []string{url}
	}

	_ = exec.Command(cmd, args...).Start()
}
