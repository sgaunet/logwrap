// Package main provides the logwrap command-line tool.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sgaunet/logwrap/pkg/apperrors"
	"github.com/sgaunet/logwrap/pkg/config"
	"github.com/sgaunet/logwrap/pkg/executor"
	"github.com/sgaunet/logwrap/pkg/filter"
	"github.com/sgaunet/logwrap/pkg/formatter"
	"github.com/sgaunet/logwrap/pkg/processor"
)

// Build-time variables injected via -ldflags.
var (
	version   = "development"
	commit    = "unknown"
	buildDate = "unknown"
)

const (
	exitCodeSIGINT         = 130 // 128 + 2 (SIGINT)
	exitCodeSIGTERM        = 143 // 128 + 15 (SIGTERM)
	gracefulShutdownTimeout = 5 * time.Second
	processorWaitTimeout    = 3 * time.Second
	usage                   = `LogWrap - Command execution wrapper with configurable log prefixes

Usage:
  logwrap [options] -- <command> [args...]
  logwrap [options] <command> [args...]

Options:
  -config string      Configuration file path
  -template string    Log prefix template (default "[{{.Timestamp}}] [{{.Level}}] [{{.User}}:{{.PID}}] ")
  -utc                Use UTC timestamps (default false)
  -colors             Enable colored output (default false)
  -format string      Output format: text, json, structured (default "text")
  -validate           Validate configuration and exit (no command needed)
  -help               Show this help message
  -version            Show version information

Template Variables:
  {{.Timestamp}}      Current timestamp (formatted using strftime format in config)
  {{.Level}}          Log level (INFO, ERROR, etc.)
  {{.User}}           Username (controlled via config file)
  {{.PID}}            Process ID (controlled via config file)

Timestamp Format (strftime):
  Uses Linux date command format (not Go time format)
  Common directives:
    %Y  Year (2024)          %m  Month (01-12)       %d  Day (01-31)
    %H  Hour 24h (00-23)     %M  Minute (00-59)      %S  Second (00-59)
    %a  Weekday short (Mon)  %A  Weekday full (Monday)
    %b  Month short (Jan)    %B  Month full (January)
    %z  Timezone offset      %f  Microseconds

  Example formats:
    %Y-%m-%d %H:%M:%S       → 2024-01-15 14:30:45
    %d/%b/%Y %H:%M          → 15/Jan/2024 14:30
    %Y-%m-%dT%H:%M:%S%z     → 2024-01-15T14:30:45-0700

Examples:
  logwrap echo "Hello World"
  logwrap -config myconfig.yaml make build
  logwrap -utc -colors make test
  logwrap -template "[{{.Timestamp}}] " ls -la
  logwrap -template "[{{.Level}}] [{{.User}}:{{.PID}}] " -- sh -c "echo stdout; echo stderr >&2"
  logwrap -validate
  logwrap -validate -config myconfig.yaml

Configuration:
  LogWrap looks for configuration files in the following order:
  1. File specified with -config flag
  2. ./logwrap.yaml or ./logwrap.yml
  3. ~/.config/logwrap/config.yaml
  4. ~/.logwrap.yaml

  To control user/PID inclusion, use a config file or customize the -template flag.

For more information, visit: https://github.com/sgaunet/logwrap`
)

func main() {
	const minArgs = 2
	if len(os.Args) < minArgs {
		fmt.Fprintf(os.Stderr, "%s\n", usage)
		os.Exit(1)
	}

	args, command, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing arguments: %v\n", err)
		os.Exit(1)
	}

	if hasFlag(args, "-help") {
		_, _ = fmt.Fprintf(os.Stdout, "%s\n", usage)
		os.Exit(0)
	}

	if hasFlag(args, "-version") {
		_, _ = fmt.Fprintf(os.Stdout, "logwrap version %s\n  commit: %s\n  built:  %s\n", version, commit, buildDate)
		os.Exit(0)
	}

	if hasFlag(args, "-validate") {
		os.Exit(validateConfig(args))
	}

	if len(command) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no command specified\n\n%s\n", usage)
		os.Exit(1)
	}

	configFile := getConfigFile(args)
	cfg, err := config.LoadConfig(configFile, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(run(cfg, command))
}

func validateConfig(args []string) int {
	// Filter out -validate before passing to LoadConfig, since it's
	// not a config flag and would be rejected by the flag parser.
	var filteredArgs []string
	for _, arg := range args {
		if arg != "-validate" && arg != "-validate=true" {
			filteredArgs = append(filteredArgs, arg)
		}
	}
	args = filteredArgs

	configFile := getConfigFile(args)

	source := configFile
	if source == "" {
		source = "(built-in defaults)"
	}

	cfg, err := config.LoadConfig(configFile, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration validation failed\n\nError: %v\n", err)
		if configFile != "" {
			fmt.Fprintf(os.Stderr, "  in file: %s\n", configFile)
		}
		return 1
	}

	_, _ = fmt.Fprintf(os.Stdout, "Configuration is valid\n\n")
	_, _ = fmt.Fprintf(os.Stdout, "Loaded from: %s\n\n", source)
	printConfigSettings(cfg)
	return 0
}

func printConfigSettings(cfg *config.Config) {
	_, _ = fmt.Fprintf(os.Stdout, "Settings:\n")
	_, _ = fmt.Fprintf(os.Stdout, "  Output format:    %s\n", cfg.Output.Format)
	_, _ = fmt.Fprintf(os.Stdout, "  Template:         %s\n", cfg.Prefix.Template)
	_, _ = fmt.Fprintf(os.Stdout, "  Timestamp format: %s\n", cfg.Prefix.Timestamp.Format)
	_, _ = fmt.Fprintf(os.Stdout, "  Timestamp UTC:    %t\n", cfg.Prefix.Timestamp.UTC)
	_, _ = fmt.Fprintf(os.Stdout, "  Colors:           %t\n", cfg.Prefix.Colors.Enabled)
	if cfg.Prefix.Colors.Enabled {
		printColorSettings(cfg)
	}
	_, _ = fmt.Fprintf(os.Stdout, "  User:             %t (%s)\n", cfg.Prefix.User.Enabled, cfg.Prefix.User.Format)
	_, _ = fmt.Fprintf(os.Stdout, "  PID:              %t (%s)\n", cfg.Prefix.PID.Enabled, cfg.Prefix.PID.Format)
	_, _ = fmt.Fprintf(os.Stdout, "  Default stdout:   %s\n", cfg.LogLevel.DefaultStdout)
	_, _ = fmt.Fprintf(os.Stdout, "  Default stderr:   %s\n", cfg.LogLevel.DefaultStderr)
	_, _ = fmt.Fprintf(os.Stdout, "  Detection:        %t\n", cfg.LogLevel.Detection.Enabled)
	if cfg.Filter.Enabled {
		printFilterSettings(cfg)
	}
}

func printColorSettings(cfg *config.Config) {
	if cfg.Prefix.Colors.Theme != "" {
		_, _ = fmt.Fprintf(os.Stdout, "    Theme:          %s\n", cfg.Prefix.Colors.Theme)
	}
	_, _ = fmt.Fprintf(os.Stdout, "    Info:           %s\n", cfg.Prefix.Colors.Info)
	_, _ = fmt.Fprintf(os.Stdout, "    Error:          %s\n", cfg.Prefix.Colors.Error)
	_, _ = fmt.Fprintf(os.Stdout, "    Timestamp:      %s\n", cfg.Prefix.Colors.Timestamp)
}

func printFilterSettings(cfg *config.Config) {
	_, _ = fmt.Fprintf(os.Stdout, "  Filter:           true\n")
	if len(cfg.Filter.IncludeLevels) > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "    Include levels: %s\n", strings.Join(cfg.Filter.IncludeLevels, ", "))
	}
	if len(cfg.Filter.ExcludeLevels) > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "    Exclude levels: %s\n", strings.Join(cfg.Filter.ExcludeLevels, ", "))
	}
	if len(cfg.Filter.IncludePatterns) > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "    Include patterns: %s\n", strings.Join(cfg.Filter.IncludePatterns, ", "))
	}
	if len(cfg.Filter.ExcludePatterns) > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "    Exclude patterns: %s\n", strings.Join(cfg.Filter.ExcludePatterns, ", "))
	}
}

func parseArgs(args []string) ([]string, []string, error) {
	var configArgs []string
	var command []string

	i := 0
	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			command = args[i+1:]
			break
		}

		if len(arg) > 0 && arg[0] == '-' {
			configArgs = append(configArgs, arg)

			if arg == "-config" || arg == "-template" || arg == "-format" {
				if i+1 >= len(args) {
					return nil, nil, fmt.Errorf("%w: %s", apperrors.ErrOptionRequiresValue, arg)
				}
				i++
				configArgs = append(configArgs, args[i])
			}
		} else {
			command = args[i:]
			break
		}

		i++
	}

	return configArgs, command, nil
}

func hasFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag || arg == flag+"=true" {
			return true
		}
	}
	return false
}

func getConfigFile(args []string) string {
	for i, arg := range args {
		if arg == "-config" && i+1 < len(args) {
			return args[i+1]
		}
		if val, ok := strings.CutPrefix(arg, "-config="); ok {
			return val
		}
	}
	return config.FindConfigFile()
}

func run(cfg *config.Config, command []string) int {
	exec, err := executor.New(command)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Execution error: failed to create executor: %v\n", err)
		return 1
	}
	defer exec.Cleanup()

	form, err := formatter.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Execution error: failed to create formatter: %v\n", err)
		return 1
	}

	var procOpts []processor.Option
	if cfg.Filter.Enabled {
		f, fErr := filter.New(filter.Config{
			Enabled:         cfg.Filter.Enabled,
			ExcludePatterns: cfg.Filter.ExcludePatterns,
			IncludePatterns: cfg.Filter.IncludePatterns,
			ExcludeLevels:   cfg.Filter.ExcludeLevels,
			IncludeLevels:   cfg.Filter.IncludeLevels,
		}, cfg.LogLevel.Detection.Keywords)
		if fErr != nil {
			fmt.Fprintf(os.Stderr, "Execution error: failed to create filter: %v\n", fErr)
			return 1
		}
		procOpts = append(procOpts, processor.WithFilter(f))
	}

	proc := processor.New(form, os.Stdout, procOpts...)

	if err := exec.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Execution error: failed to start command: %v\n", err)
		return 1
	}

	stdout, stderr := exec.GetStreams()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start stream processing in background
	ctx := context.Background()
	processingDone := make(chan error, 1)
	go func() {
		processingDone <- proc.ProcessStreams(ctx, stdout, stderr)
	}()

	// Wait for command to complete or signal
	receivedSignal, cmdErr := waitForCommandOrSignal(exec, proc, sigChan)

	// Wait for stream processing to complete
	waitForProcessing(proc, processingDone)

	// Clean up signal handler before exit
	signal.Stop(sigChan)

	return determineExitCode(exec, receivedSignal, cmdErr)
}

func waitForCommandOrSignal(
	exec *executor.Executor,
	proc *processor.Processor,
	sigChan chan os.Signal,
) (os.Signal, error) {
	cmdDone := make(chan error, 1)
	go func() {
		cmdDone <- exec.Wait()
	}()

	var receivedSignal os.Signal
	var cmdErr error

	select {
	case sig := <-sigChan:
		receivedSignal = sig
		cmdErr = handleSignalShutdown(exec, proc, sig, cmdDone)
	case cmdErr = <-cmdDone:
		// Command finished normally
	}

	return receivedSignal, cmdErr
}

func handleSignalShutdown(exec *executor.Executor, proc *processor.Processor, sig os.Signal, cmdDone chan error) error {
	fmt.Fprintf(os.Stderr, "\nReceived signal %v, initiating graceful shutdown...\n", sig)

	// Signal the child process first so it can produce cleanup output.
	// The processor keeps running to capture any final output from the child.
	if err := exec.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to stop executor gracefully: %v\n", err)
	}

	// Wait for command with timeout
	shutdownTimer := time.NewTimer(gracefulShutdownTimeout)
	defer shutdownTimer.Stop()

	select {
	case cmdErr := <-cmdDone:
		// Command finished gracefully. Processor will finish naturally
		// when the child's pipes close.
		return cmdErr
	case <-shutdownTimer.C:
		fmt.Fprintf(os.Stderr, "Shutdown timeout exceeded, forcing kill...\n")
		if err := exec.Kill(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to kill process: %v\n", err)
		}
		proc.Stop()
		return <-cmdDone // Wait for process to actually die
	}
}

func waitForProcessing(proc *processor.Processor, processingDone chan error) {
	processingTimer := time.NewTimer(processorWaitTimeout)
	defer processingTimer.Stop()

	select {
	case procErr := <-processingDone:
		if procErr != nil {
			fmt.Fprintf(os.Stderr, "Stream processing error: %v\n", procErr)
		}
	case <-processingTimer.C:
		fmt.Fprintf(os.Stderr, "Warning: stream processing timeout, some output may be lost\n")
		proc.Stop() // Ensure processor is stopped
	}
}

func determineExitCode(exec *executor.Executor, receivedSignal os.Signal, _ error) int {
	// If we received a signal, use signal-based exit code
	if receivedSignal != nil {
		switch receivedSignal {
		case syscall.SIGINT:
			return exitCodeSIGINT
		case syscall.SIGTERM:
			return exitCodeSIGTERM
		}
	}

	// Otherwise use command's exit code
	return exec.GetExitCode()
}
