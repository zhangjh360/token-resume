package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"tokenresume/internal/config"
	"tokenresume/internal/monitor"
	"tokenresume/internal/ratelimit"
	"tokenresume/internal/ratelimit/provider"
	"tokenresume/internal/resume"
	"tokenresume/pkg/logger"
)

func main() {
	handled, pid, err := parseResumeCommand(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if handled {
		if err := monitor.ResumeProcess(pid); err != nil {
			fmt.Fprintf(os.Stderr, "resume process failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("process %d resumed\n", pid)
		return
	}

	var (
		configPath = flag.String("config", "config.yaml", "config file path")
		daemon     = flag.Bool("daemon", false, "run as daemon mode")
	)
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Logging.Level)
	if *daemon {
		log.Info("daemon mode enabled")
	}

	pm, err := monitor.NewProcessMonitor(cfg.Monitor.Processes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create process monitor failed: %v\n", err)
		os.Exit(1)
	}

	providerClient := provider.New(cfg.RateLimit)
	detector := ratelimit.NewDetector(providerClient, cfg.RateLimit.Fallback)
	manager := resume.NewManager(cfg.Resume, log)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	run(ctx, cfg, pm, detector, manager, log)
}

func parseResumeCommand(args []string) (bool, int, error) {
	if len(args) == 0 || args[0] != "resume" {
		return false, 0, nil
	}
	resumeFlags := flag.NewFlagSet("resume", flag.ContinueOnError)
	resumeFlags.SetOutput(os.Stderr)
	pid := resumeFlags.Int("pid", 0, "pid for manual resume command")
	if err := resumeFlags.Parse(args[1:]); err != nil {
		return true, 0, err
	}
	if *pid <= 0 {
		return true, 0, errors.New("resume command requires --pid")
	}
	return true, *pid, nil
}

func run(
	ctx context.Context,
	cfg *config.Config,
	pm *monitor.ProcessMonitor,
	detector *ratelimit.Detector,
	manager *resume.Manager,
	log *logger.Logger,
) {
	processTicker := time.NewTicker(cfg.Monitor.PollInterval)
	tokenTicker := time.NewTicker(cfg.Monitor.TokenCheckInterval)
	defer processTicker.Stop()
	defer tokenTicker.Stop()

	tracked := make(map[int]monitor.ProcessInfo)
	var mu sync.Mutex

	scan := func() {
		procs, err := pm.ListTargetProcesses()
		if err != nil {
			log.Error("scan target processes failed: %v", err)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		next := make(map[int]monitor.ProcessInfo, len(procs))
		for _, p := range procs {
			next[p.PID] = p
			if _, ok := tracked[p.PID]; !ok {
				log.Info("found target process pid=%d command=%s", p.PID, p.Command)
			}
		}
		tracked = next
	}

	handleRateLimit := func() {
		status, err := detector.Check(ctx)
		if err != nil {
			log.Error("check rate limit failed: %v", err)
			return
		}
		if !status.IsLimited {
			return
		}

		log.Warn("rate limited: remaining=%d resetAt=%s", status.RemainingTokens, status.ResetAt.Format(time.RFC3339))

		mu.Lock()
		targets := make([]monitor.ProcessInfo, 0, len(tracked))
		for _, p := range tracked {
			targets = append(targets, p)
		}
		mu.Unlock()

		if len(targets) == 0 {
			log.Warn("no tracked process to suspend")
			return
		}

		for _, p := range targets {
			snapshot, snapErr := pm.BuildSnapshot(p)
			if snapErr != nil {
				log.Error("build snapshot failed pid=%d err=%v", p.PID, snapErr)
				continue
			}
			if err := manager.SaveSnapshot(snapshot); err != nil {
				log.Error("save snapshot failed pid=%d err=%v", p.PID, err)
			}
			if err := pm.SuspendProcess(p.PID); err != nil {
				log.Error("suspend process failed pid=%d err=%v", p.PID, err)
				continue
			}
			log.Info("suspended process pid=%d", p.PID)
		}

		if err := detector.WaitForReset(ctx, status.ResetAt, cfg.Resume.SafetyMarginSeconds); err != nil {
			log.Error("wait for reset failed: %v", err)
			return
		}

		snapshots := manager.ListSnapshots()
		for _, s := range snapshots {
			if err := manager.Resume(ctx, s); err != nil {
				log.Error("resume failed pid=%d strategy=%s err=%v", s.PID, manager.Strategy(), err)
				continue
			}
			log.Info("resumed task pid=%d strategy=%s", s.PID, manager.Strategy())
			if err := manager.RemoveSnapshot(s.PID); err != nil {
				log.Error("remove snapshot failed pid=%d err=%v", s.PID, err)
			}
		}
	}

	scan()

	for {
		select {
		case <-ctx.Done():
			log.Info("shutdown requested")
			return
		case <-processTicker.C:
			scan()
		case <-tokenTicker.C:
			handleRateLimit()
		}
	}
}
