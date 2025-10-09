// Package main provides the logwrap command-line tool.
package main

import (
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/sgaunet/logwrap/pkg/config"
	"github.com/sgaunet/logwrap/pkg/errors"
	"github.com/sgaunet/logwrap/pkg/executor"
	"github.com/sgaunet/logwrap/pkg/formatter"
	"github.com/sgaunet/logwrap/pkg/processor"
)

const (
	version = "development"
	usage   = `LogWrap - Command execution wrapper with configurable log prefixes

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
  {{.Timestamp}}      Current timestamp
  {{.Level}}          Log level (INFO, ERROR, etc.)
  {{.User}}           Username (controlled via config file)
  {{.PID}}            Process ID (controlled via config file)

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
					return nil, nil, fmt.Errorf("%w: %s", errors.ErrOptionRequiresValue, arg)
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

	ctx := context.Background()
	processingDone := make(chan error, 1)
	go func() {
		processingDone <- proc.ProcessStreams(ctx, stdout, stderr)
	}()

	if err := exec.Wait(); err != nil {
		return fmt.Errorf("command execution error: %w", err)
	}

	if err := <-processingDone; err != nil {
		fmt.Fprintf(os.Stderr, "Stream processing error: %v\n", err)
	}

	os.Exit(exec.GetExitCode())
	return nil
}
