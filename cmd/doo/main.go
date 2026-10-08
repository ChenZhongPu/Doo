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

const sharkArt = `                     /|
                    / |                      /|
                   /  |                     / |
     _..----------'   '---..._______       /  |
   .-'                              '-._  /  /
 .'    o       ) ) )                    '/  /
|              ) ) )                       <
 '.  \/\/\/\/\/) ) )                     \  \
   '-.                             _..--''\  \
      '--..____________________..-'        \__\
                  \ \
                   \_\`

func printBanner(listenAddr string, maxTargets int, dataDir string) {
	cyan := "\033[36m"
	bold := "\033[1m"
	green := "\033[32m"
	gray := "\033[90m"
	reset := "\033[0m"

	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		cyan, bold, green, gray, reset = "", "", "", "", ""
	}

	targetsInfo := fmt.Sprintf("%d", maxTargets)
	if maxTargets <= 0 {
		targetsInfo = "unlimited"
	}

	commitInfo := gitCommit
	if commitInfo == "" {
		commitInfo = "dev"
	}

	fmt.Printf("\n%s%s%s\n\n", cyan, sharkArt, reset)
	fmt.Printf("  %s%sDoo - Multi-LLM Playground%s\n", bold, cyan, reset)
	fmt.Printf("  %sVersion:%s     %s %s(commit: %s, built: %s)%s\n", gray, reset, version, gray, commitInfo, buildTime, reset)
	if listenAddr != "" {
		fmt.Printf("  %sServer URL:%s  %s%shttp://%s%s\n", gray, reset, bold, green, listenAddr, reset)
	}
	if maxTargets >= 0 {
		fmt.Printf("  %sMax Models:%s  %s\n", gray, reset, targetsInfo)
	}
	if dataDir != "" {
		fmt.Printf("  %sData Dir:%s    %s\n", gray, reset, dataDir)
	}
	fmt.Println()
}

func main() {
	var (
		hostFlag       = flag.String("host", "localhost", "HTTP server listen host")
		portFlag       = flag.Int("port", 8787, "HTTP server listen port")
		addrFlag       = flag.String("addr", "", "HTTP server listen address [host:port] (overrides -host and -port if set)")
		maxTargetsFlag = flag.Int("max-targets", 8, "Maximum number of models to compare concurrently (0 for unlimited)")
		dataFlag       = flag.String("data", "~/.doo", "Data directory path for database and keys")
		openFlag       = flag.Bool("open", true, "Open browser after server starts (default true, use -open=false to disable)")
		versionFlag    = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	if *versionFlag {
		printBanner("", *maxTargetsFlag, "")
		os.Exit(0)
	}

	listenAddr := *addrFlag
	if listenAddr == "" {
		listenAddr = fmt.Sprintf("%s:%d", *hostFlag, *portFlag)
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

	warnIfNotLoopback(listenAddr)

	secCfg := api.SecurityConfig{BindAddr: listenAddr}
	apiServer := api.NewServer(st, keyStore, version, buildTime, gitCommit, secCfg, *maxTargetsFlag)

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
		Addr:    listenAddr,
		Handler: api.SecurityMiddleware(secCfg, topMux),
	}

	printBanner(listenAddr, *maxTargetsFlag, dataDir)

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	if *openFlag {
		go func() {
			time.Sleep(150 * time.Millisecond)
			browserURL := fmt.Sprintf("http://%s", listenAddr)
			if h, p, err := net.SplitHostPort(listenAddr); err == nil {
				if h == "" || h == "0.0.0.0" || h == "::" {
					browserURL = fmt.Sprintf("http://localhost:%s", p)
				}
			}
			openBrowser(browserURL)
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
