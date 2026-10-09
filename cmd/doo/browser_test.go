package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestLaunchBrowser(t *testing.T) {
	const url = "http://localhost:8787/#/compare?note=a&other=b"
	failure := errors.New("launcher failed")
	tests := []struct {
		name, goos, display, wayland string
		failures                     []error
		wantCommands                 [][]string
		wantError                    bool
	}{
		{name: "KDE or other Linux desktop uses GIO", goos: "linux", display: ":1", wantCommands: [][]string{{"gio", "open", url}}},
		{name: "Wayland needs no X11 display", goos: "linux", wayland: "wayland-0", wantCommands: [][]string{{"gio", "open", url}}},
		{name: "missing GIO falls back", goos: "linux", display: ":1", failures: []error{os.ErrNotExist}, wantCommands: [][]string{{"gio", "open", url}, {"xdg-open", url}}},
		{name: "failed GIO falls back", goos: "linux", display: ":1", failures: []error{failure}, wantCommands: [][]string{{"gio", "open", url}, {"xdg-open", url}}},
		{name: "both launchers fail", goos: "linux", display: ":1", failures: []error{failure, failure}, wantCommands: [][]string{{"gio", "open", url}, {"xdg-open", url}}, wantError: true},
		{name: "headless does not invoke a launcher", goos: "linux", wantError: true},
		{name: "macOS", goos: "darwin", wantCommands: [][]string{{"open", url}}},
		{name: "Windows", goos: "windows", wantCommands: [][]string{{"rundll32", "url.dll,FileProtocolHandler", url}}},
		{name: "BSD", goos: "freebsd", wantCommands: [][]string{{"xdg-open", url}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var commands [][]string
			getenv := func(name string) string {
				switch name {
				case "DISPLAY":
					return tt.display
				case "WAYLAND_DISPLAY":
					return tt.wayland
				}
				return ""
			}
			err := launchBrowser(url, tt.goos, getenv, func(name string, args ...string) error {
				commands = append(commands, append([]string{name}, args...))
				if len(commands) <= len(tt.failures) {
					return tt.failures[len(commands)-1]
				}
				return nil
			})
			if (err != nil) != tt.wantError {
				t.Fatalf("error = %v, wantError = %v", err, tt.wantError)
			}
			if !reflect.DeepEqual(commands, tt.wantCommands) {
				t.Fatalf("commands = %v, want %v", commands, tt.wantCommands)
			}
			if tt.name == "both launchers fail" && (!errors.Is(err, failure) || !strings.Contains(err.Error(), "gio:") || !strings.Contains(err.Error(), "xdg-open:")) {
				t.Fatalf("launcher errors not preserved: %v", err)
			}
		})
	}
}

func TestOpenBrowserWaitsForExitAndFallsBack(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux launcher regression")
	}
	dir := t.TempDir()
	trace := filepath.Join(dir, "trace")
	t.Setenv("PATH", dir)
	t.Setenv("DISPLAY", ":1")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DOO_BROWSER_TEST_TRACE", trace)
	for name, script := range map[string]string{
		"gio":      "#!/bin/sh\nprintf 'gio\\n' >> \"$DOO_BROWSER_TEST_TRACE\"\nexit 1\n",
		"xdg-open": "#!/bin/sh\nprintf 'xdg-open\\n' >> \"$DOO_BROWSER_TEST_TRACE\"\nexit 0\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := openBrowser("http://localhost:8787"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "gio\nxdg-open\n" {
		t.Fatalf("launcher trace = %q; expected failed GIO to finish before fallback", got)
	}
}
