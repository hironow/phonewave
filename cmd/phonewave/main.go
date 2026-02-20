package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/hironow/phonewave"
)

var version = "dev"

// knownSubcommands lists all recognized subcommands.
var knownSubcommands = map[string]bool{
	"init":   true,
	"add":    true,
	"remove": true,
	"sync":   true,
	"doctor": true,
	"run":    true,
	"status": true,
}

// extractSubcommand separates args (os.Args[1:]) into a subcommand and
// remaining positional/flag arguments. This allows flexible ordering:
//
//	phonewave init ./repo-a ./repo-b
//	phonewave add ./repo-c
//	phonewave --version
//	phonewave run --verbose
func extractSubcommand(args []string) (subcmd string, rest []string) {
	if len(args) == 0 {
		return "", nil
	}

	// Check for --version / --help before subcommand
	if args[0] == "--version" || args[0] == "-v" {
		return "version", nil
	}
	if args[0] == "--help" || args[0] == "-h" {
		return "help", nil
	}

	if knownSubcommands[args[0]] {
		return args[0], args[1:]
	}

	// No recognized subcommand — treat all as args to default behavior
	return "", args
}

// isFlagArg reports whether s looks like a flag argument.
func isFlagArg(s string) bool {
	return strings.HasPrefix(s, "-")
}

// extractPaths extracts non-flag positional arguments from args.
func extractPaths(args []string) []string {
	var paths []string
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if !isFlagArg(arg) {
			paths = append(paths, arg)
		}
	}
	return paths
}

func main() {
	os.Exit(run())
}

func run() int {
	subcmd, rest := extractSubcommand(os.Args[1:])

	switch subcmd {
	case "version":
		fmt.Printf("phonewave %s\n", version)
		return 0
	case "help", "":
		if subcmd == "" && len(rest) == 0 {
			printUsage()
			return 0
		}
		if subcmd == "help" {
			printUsage()
			return 0
		}
		// Fall through to unknown handling for non-empty rest with no subcmd
		fmt.Fprintf(os.Stderr, "Error: unknown arguments: %s\n", strings.Join(rest, " "))
		printUsage()
		return 1
	case "init":
		return runInit(rest)
	case "add":
		return runAdd(rest)
	case "remove":
		return runRemove(rest)
	case "sync":
		return runSync(rest)
	case "doctor":
		return runDoctor(rest)
	case "run":
		return runDaemon(rest)
	case "status":
		return runStatus(rest)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command %q\n", subcmd)
		printUsage()
		return 1
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `phonewave %s — D-Mail courier daemon

Usage:
  phonewave <command> [arguments]

Commands:
  init <repo-path...>    Scan repositories, discover tools, generate routing table
  add <repo-path>        Add a new repository to the ecosystem
  remove <repo-path>     Remove a repository from the ecosystem
  sync                   Re-scan all repositories, reconcile routing table
  doctor                 Verify ecosystem health
  run                    Start the courier daemon
  status                 Show daemon and delivery status

Options:
  --version              Show version and exit
  --help                 Show this help

`, version)
}

func runInit(args []string) int {
	paths := extractPaths(args)
	if len(paths) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: phonewave init <repo-path> [repo-path...]\n")
		return 1
	}

	result, err := phonewave.Init(paths)
	if err != nil {
		phonewave.LogError("%v", err)
		return 1
	}

	configPath := filepath.Join(".", phonewave.ConfigFile)
	if err := phonewave.WriteConfig(configPath, result.Config); err != nil {
		phonewave.LogError("write config: %v", err)
		return 1
	}

	if err := phonewave.EnsureStateDir("."); err != nil {
		phonewave.LogError("create state dir: %v", err)
		return 1
	}

	// Print summary
	phonewave.LogOK("Scanned %d repositories", result.RepoCount)
	for _, repo := range result.Config.Repositories {
		for _, ep := range repo.Endpoints {
			phonewave.LogOK("  %s/%s  produces=%v consumes=%v", filepath.Base(repo.Path), ep.Dir, ep.Produces, ep.Consumes)
		}
	}
	phonewave.LogOK("Derived %d routes", len(result.Config.Routes))
	for _, r := range result.Config.Routes {
		phonewave.LogInfo("  %s: %s → %v", r.Kind, r.From, r.To)
	}

	printOrphanWarnings(result.Orphans)

	phonewave.LogOK("Config written to %s", configPath)
	return 0
}

func runAdd(args []string) int {
	paths := extractPaths(args)
	if len(paths) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: phonewave add <repo-path>\n")
		return 1
	}

	configPath := filepath.Join(".", phonewave.ConfigFile)
	cfg, err := phonewave.LoadConfig(configPath)
	if err != nil {
		phonewave.LogError("load config: %v", err)
		phonewave.LogInfo("Run 'phonewave init' first")
		return 1
	}

	orphans, err := phonewave.Add(cfg, paths[0])
	if err != nil {
		phonewave.LogError("%v", err)
		return 1
	}

	if err := phonewave.WriteConfig(configPath, cfg); err != nil {
		phonewave.LogError("write config: %v", err)
		return 1
	}

	absPath, _ := filepath.Abs(paths[0])
	phonewave.LogOK("Added %s", absPath)
	phonewave.LogOK("%d routes total", len(cfg.Routes))
	printOrphanWarnings(*orphans)

	return 0
}

func runRemove(args []string) int {
	paths := extractPaths(args)
	if len(paths) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: phonewave remove <repo-path>\n")
		return 1
	}

	configPath := filepath.Join(".", phonewave.ConfigFile)
	cfg, err := phonewave.LoadConfig(configPath)
	if err != nil {
		phonewave.LogError("load config: %v", err)
		return 1
	}

	orphans, err := phonewave.Remove(cfg, paths[0])
	if err != nil {
		phonewave.LogError("%v", err)
		return 1
	}

	if err := phonewave.WriteConfig(configPath, cfg); err != nil {
		phonewave.LogError("write config: %v", err)
		return 1
	}

	absPath, _ := filepath.Abs(paths[0])
	phonewave.LogOK("Removed %s", absPath)
	phonewave.LogOK("%d routes remaining", len(cfg.Routes))
	printOrphanWarnings(*orphans)

	return 0
}

func runSync(_ []string) int {
	configPath := filepath.Join(".", phonewave.ConfigFile)
	cfg, err := phonewave.LoadConfig(configPath)
	if err != nil {
		phonewave.LogError("load config: %v", err)
		phonewave.LogInfo("Run 'phonewave init' first")
		return 1
	}

	orphans, err := phonewave.Sync(cfg)
	if err != nil {
		phonewave.LogError("sync: %v", err)
		return 1
	}

	if err := phonewave.WriteConfig(configPath, cfg); err != nil {
		phonewave.LogError("write config: %v", err)
		return 1
	}

	phonewave.LogOK("Synced %d repositories, %d routes", len(cfg.Repositories), len(cfg.Routes))
	printOrphanWarnings(*orphans)

	return 0
}

func runDoctor(_ []string) int {
	configPath := filepath.Join(".", phonewave.ConfigFile)
	cfg, err := phonewave.LoadConfig(configPath)
	if err != nil {
		phonewave.LogError("load config: %v", err)
		phonewave.LogInfo("Run 'phonewave init' first")
		return 1
	}

	stateDir := filepath.Join(".", phonewave.StateDir)
	report := phonewave.Doctor(cfg, stateDir)

	for _, issue := range report.Issues {
		switch issue.Severity {
		case "ok":
			phonewave.LogOK("%s  %s", issue.Endpoint, issue.Message)
		case "fixed":
			phonewave.LogWarn("%s  %s", issue.Endpoint, issue.Message)
		case "warn":
			phonewave.LogWarn("%s  %s", issue.Endpoint, issue.Message)
		case "error":
			phonewave.LogError("%s  %s", issue.Endpoint, issue.Message)
		}
	}

	// Daemon status
	if report.DaemonStatus.Running {
		phonewave.LogOK("Daemon: running (PID %d)", report.DaemonStatus.PID)
	} else {
		phonewave.LogOK("Daemon: not running")
	}

	if report.Healthy {
		phonewave.LogOK("Ecosystem healthy")
		return 0
	}
	phonewave.LogError("Ecosystem has issues")
	return 1
}

func runDaemon(args []string) int {
	verbose := false
	dryRun := false
	for _, arg := range args {
		switch arg {
		case "--verbose", "-v":
			verbose = true
		case "--dry-run":
			dryRun = true
		}
	}

	configPath := filepath.Join(".", phonewave.ConfigFile)
	cfg, err := phonewave.LoadConfig(configPath)
	if err != nil {
		phonewave.LogError("load config: %v", err)
		phonewave.LogInfo("Run 'phonewave init' first")
		return 1
	}

	routes, err := phonewave.ResolveRoutes(cfg)
	if err != nil {
		phonewave.LogError("resolve routes: %v", err)
		return 1
	}

	outboxDirs := phonewave.CollectOutboxDirs(cfg)
	if len(outboxDirs) == 0 {
		phonewave.LogWarn("No outbox directories to watch")
		return 0
	}

	stateDir := filepath.Join(".", phonewave.StateDir)
	if err := phonewave.EnsureStateDir("."); err != nil {
		phonewave.LogError("create state dir: %v", err)
		return 1
	}

	d, err := phonewave.NewDaemon(phonewave.DaemonOptions{
		Routes:     routes,
		OutboxDirs: outboxDirs,
		StateDir:   stateDir,
		Verbose:    verbose,
		DryRun:     dryRun,
	})
	if err != nil {
		phonewave.LogError("create daemon: %v", err)
		return 1
	}

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		if verbose {
			phonewave.LogInfo("Received %s, shutting down...", sig)
		}
		cancel()
	}()

	phonewave.LogOK("phonewave daemon starting (%d routes, %d outboxes)", len(routes), len(outboxDirs))

	if err := d.Run(ctx); err != nil {
		phonewave.LogError("daemon: %v", err)
		return 1
	}

	phonewave.LogOK("Daemon stopped")
	return 0
}

func runStatus(_ []string) int {
	configPath := filepath.Join(".", phonewave.ConfigFile)
	cfg, err := phonewave.LoadConfig(configPath)
	if err != nil {
		phonewave.LogError("load config: %v", err)
		phonewave.LogInfo("Run 'phonewave init' first")
		return 1
	}

	stateDir := filepath.Join(".", phonewave.StateDir)
	status := phonewave.Status(cfg, stateDir)

	fmt.Fprintf(os.Stderr, "phonewave status:\n")
	if status.DaemonRunning {
		fmt.Fprintf(os.Stderr, "  Daemon:    running (PID %d)\n", status.DaemonPID)
	} else {
		fmt.Fprintf(os.Stderr, "  Daemon:    stopped\n")
	}
	fmt.Fprintf(os.Stderr, "  Watching:  %d outbox directories across %d repositories\n", status.OutboxCount, status.RepoCount)
	fmt.Fprintf(os.Stderr, "  Routes:    %d\n", status.RouteCount)
	fmt.Fprintf(os.Stderr, "  Pending:   %d items in error queue\n", status.PendingErrors)

	return 0
}

func printOrphanWarnings(orphans phonewave.OrphanReport) {
	for _, kind := range orphans.UnconsumedKinds {
		phonewave.LogWarn("Orphaned: kind=%q is produced but not consumed", kind)
	}
	for _, kind := range orphans.UnproducedKinds {
		phonewave.LogWarn("Orphaned: kind=%q is consumed but not produced", kind)
	}
}
