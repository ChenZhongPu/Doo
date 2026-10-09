package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func openBrowser(url string) error {
	return launchBrowser(url, runtime.GOOS, os.Getenv, func(name string, args ...string) error {
		cmd := exec.Command(name, args...)
		// Keep launcher diagnostics visible. Run also reaps the child and catches
		// failures after startup, which Start alone cannot report.
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
}

func launchBrowser(url, goos string, getenv func(string) string, run func(string, ...string) error) error {
	var commands [][]string
	switch goos {
	case "windows":
		commands = [][]string{{"rundll32", "url.dll,FileProtocolHandler", url}}
	case "darwin":
		commands = [][]string{{"open", url}}
	case "linux":
		if getenv("DISPLAY") == "" && getenv("WAYLAND_DISPLAY") == "" {
			return errors.New("no graphical session: DISPLAY and WAYLAND_DISPLAY are unset")
		}
		// GIO respects the default URI handler without starting KDE's Qt-based
		// kde-open helper. Keep xdg-open for systems without a working GIO opener.
		commands = [][]string{{"gio", "open", url}, {"xdg-open", url}}
	default:
		commands = [][]string{{"xdg-open", url}}
	}

	var failures []error
	for _, command := range commands {
		if err := run(command[0], command[1:]...); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", command[0], err))
			continue
		}
		return nil
	}
	return errors.Join(failures...)
}
