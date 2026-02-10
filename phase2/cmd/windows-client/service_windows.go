//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/fruitsalade/fruitsalade/phase2/internal/winclient"
	"github.com/fruitsalade/fruitsalade/shared/pkg/logger"
)

const serviceName = "FruitSalade"
const serviceDisplayName = "FruitSalade File Sync"
const serviceDescription = "FruitSalade on-demand file synchronization service"

type fruitService struct {
	mode       string
	syncRoot   string
	server     string
	token      string
	cacheDir   string
	maxCache   int64
	refresh    time.Duration
	watchSSE   bool
	healthChk  time.Duration
	verifyHash bool
}

func (s *fruitService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}

	cfg := winclient.CoreConfig{
		ServerURL:         s.server,
		AuthToken:         s.token,
		CacheDir:          s.cacheDir,
		SyncRoot:          s.syncRoot,
		MaxCacheSize:      s.maxCache,
		RefreshInterval:   s.refresh,
		HealthCheckPeriod: s.healthChk,
		WatchSSE:          s.watchSSE,
		VerifyHash:        s.verifyHash,
	}

	core, err := winclient.NewClientCore(cfg)
	if err != nil {
		logger.Error("Service init failed: %v", err)
		return false, 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := core.FetchMetadata(ctx); err != nil {
		logger.Error("Service metadata fetch failed: %v", err)
		return false, 1
	}

	backend := selectBackend(s.mode, s.syncRoot)

	errCh := make(chan error, 1)
	go func() {
		errCh <- backend.Start(ctx, core)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				backend.Stop()
				return false, 0
			case svc.Interrogate:
				changes <- c.CurrentStatus
			}
		case err := <-errCh:
			if err != nil {
				logger.Error("Service backend error: %v", err)
				return false, 1
			}
			return false, 0
		}
	}
}

func isWindowsService() bool {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return false
	}
	return isService
}

func runAsService(mode, syncRoot, server, token, cacheDir string,
	maxCache int64, refresh time.Duration, watchSSE bool,
	healthCheck time.Duration, verifyHash bool) {

	svcHandler := &fruitService{
		mode:       mode,
		syncRoot:   syncRoot,
		server:     server,
		token:      token,
		cacheDir:   cacheDir,
		maxCache:   maxCache,
		refresh:    refresh,
		watchSSE:   watchSSE,
		healthChk:  healthCheck,
		verifyHash: verifyHash,
	}

	if err := svc.Run(serviceName, svcHandler); err != nil {
		logger.Error("Service failed: %v", err)
		os.Exit(1)
	}
}

func doInstallService() {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot determine executable path: %v\n", err)
		os.Exit(1)
	}

	m, err := mgr.Connect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot connect to service manager: %v\n", err)
		os.Exit(1)
	}
	defer m.Disconnect()

	// Pass through all flags except install/uninstall-service
	args := filterServiceFlags(os.Args[1:])

	s, err := m.CreateService(serviceName, exePath, mgr.Config{
		DisplayName: serviceDisplayName,
		Description: serviceDescription,
		StartType:   mgr.StartAutomatic,
	}, args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create service: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	fmt.Printf("Service %q installed successfully.\n", serviceName)
	fmt.Println("Start with: sc start FruitSalade")
}

func doUninstallService() {
	m, err := mgr.Connect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot connect to service manager: %v\n", err)
		os.Exit(1)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open service: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	if err := s.Delete(); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot delete service: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Service %q uninstalled successfully.\n", serviceName)
}

func filterServiceFlags(args []string) []string {
	var filtered []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-install-service") || strings.HasPrefix(arg, "-uninstall-service") {
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered
}
