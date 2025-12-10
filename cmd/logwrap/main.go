// Package main provides the logwrap command-line tool.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/sgaunet/logwrap/pkg/apperrors"
	"github.com/sgaunet/logwrap/pkg/config"
	"github.com/sgaunet/logwrap/pkg/executor"
	"github.com/sgaunet/logwrap/pkg/formatter"
	"github.com/sgaunet/logwrap/pkg/processor"
)

const (
	version                = "development"
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
		_, _ = fmt.Fprintf(os.Stdout, "logwrap version %s\n", version)
		os.Exit(0)
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

	if err := run(cfg, command); err != nil {
		fmt.Fprintf(os.Stderr, "Execution error: %v\n", err)
		os.Exit(1)
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

		if arg[0] == '-' {
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
	return slices.Contains(args, flag)
}

func getConfigFile(args []string) string {
	for i, arg := range args {
		if arg == "-config" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return config.FindConfigFile()
}

func run(cfg *config.Config, command []string) error {
	exec, err := executor.New(command)
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	form, err := formatter.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create formatter: %w", err)
	}

	proc := processor.New(form, os.Stdout)

	if err := exec.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
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

	// Determine final exit code and exit
	exitCode := determineExitCode(exec, receivedSignal, cmdErr)
	os.Exit(exitCode)
	return nil
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

	// Stop the processor first
	proc.Stop()

	// Try to stop the executor gracefully
	if err := exec.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to stop executor gracefully: %v\n", err)
	}

	// Wait for command with timeout
	shutdownTimer := time.NewTimer(gracefulShutdownTimeout)
	defer shutdownTimer.Stop()

	select {
	case cmdErr := <-cmdDone:
		// Command finished gracefully
		return cmdErr
	case <-shutdownTimer.C:
		fmt.Fprintf(os.Stderr, "Shutdown timeout exceeded, forcing kill...\n")
		if err := exec.Kill(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to kill process: %v\n", err)
		}
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
