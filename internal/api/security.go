package api

import (
	"net"
	"net/http"
	"net/url"
)

type SecurityConfig struct {
	BindAddr string // e.g. "127.0.0.1:8787"
}

func SecurityMiddleware(cfg SecurityConfig, next http.Handler) http.Handler {
	allowedHosts := map[string]bool{
		"localhost": true,
		"127.0.0.1": true,
		"::1":       true,
		"[::1]":     true,
	}

	if cfg.BindAddr != "" {
		host, _, err := net.SplitHostPort(cfg.BindAddr)
		if err == nil && host != "" {
			allowedHosts[host] = true
		} else {
			allowedHosts[cfg.BindAddr] = true
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqHost := r.Host
		hostOnly := reqHost
		if h, _, err := net.SplitHostPort(reqHost); err == nil {
			hostOnly = h
		}

		if !allowedHosts[hostOnly] {
			http.Error(w, "Forbidden: invalid Host header", http.StatusForbidden)
			return
		}

		// CSRF check for state-changing requests
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			origin := r.Header.Get("Origin")
			if origin != "" {
				u, err := url.Parse(origin)
				if err != nil {
					http.Error(w, "Forbidden: invalid Origin header", http.StatusForbidden)
					return
				}
				originHost := u.Hostname()
				if !allowedHosts[originHost] && originHost != hostOnly {
					http.Error(w, "Forbidden: cross-origin request not allowed", http.StatusForbidden)
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}
