//go:build !windows

package main

import (
	"fmt"
	"os"
	"time"
)

func isWindowsService() bool {
	return false
}

func runAsService(mode, syncRoot, server, token, cacheDir string,
	maxCache int64, refresh time.Duration, watchSSE bool,
	healthCheck time.Duration, verifyHash bool) {
	fmt.Fprintln(os.Stderr, "Windows service mode is only available on Windows.")
	os.Exit(1)
}

func doInstallService() {
	fmt.Fprintln(os.Stderr, "Service install is only available on Windows.")
	os.Exit(1)
}

func doUninstallService() {
	fmt.Fprintln(os.Stderr, "Service uninstall is only available on Windows.")
	os.Exit(1)
}
