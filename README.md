# LogWrap

[![Go Report Card](https://goreportcard.com/badge/github.com/sgaunet/logwrap)](https://goreportcard.com/report/github.com/sgaunet/logwrap)
[![GitHub release](https://img.shields.io/github/release/sgaunet/logwrap.svg)](https://github.com/sgaunet/logwrap/releases/latest)
![GitHub Downloads](https://img.shields.io/github/downloads/sgaunet/logwrap/total)
![Coverage Badge](https://raw.githubusercontent.com/wiki/sgaunet/logwrap/coverage-badge.svg)
[![linter](https://github.com/sgaunet/logwrap/actions/workflows/linter.yml/badge.svg)](https://github.com/sgaunet/logwrap/actions/workflows/linter.yml)
[![coverage](https://github.com/sgaunet/logwrap/actions/workflows/coverage.yml/badge.svg)](https://github.com/sgaunet/logwrap/actions/workflows/coverage.yml)
[![Snapshot Build](https://github.com/sgaunet/logwrap/actions/workflows/snapshot.yml/badge.svg)](https://github.com/sgaunet/logwrap/actions/workflows/snapshot.yml)
[![Release Build](https://github.com/sgaunet/logwrap/actions/workflows/release.yml/badge.svg)](https://github.com/sgaunet/logwrap/actions/workflows/release.yml)
[![GoDoc](https://godoc.org/github.com/sgaunet/logwrap?status.svg)](https://godoc.org/github.com/sgaunet/logwrap)
[![License](https://img.shields.io/github/license/sgaunet/logwrap.svg)](LICENSE)

LogWrap is a command execution wrapper that adds configurable prefixes to log output streams. It intercepts stdout and stderr from executed commands and processes them in real-time with customizable formatting including timestamps, log levels, colors, user information, and process IDs.

## Features

- **Real-time processing**: No buffering delays, immediate output
- **Configurable prefixes**: Timestamps, log levels, colors, user info, PID
- **Stream separation**: Distinguish between stdout (INFO) and stderr (ERROR)
- **Flexible configuration**: YAML config files + CLI flag overrides
- **Log level detection**: Automatic detection based on keywords
- **Color support**: ANSI color codes for enhanced readability (disabled by default)
- **Signal handling**: Clean shutdown and process management
- **Multiple output formats**: Text, JSON, structured (planned)

## Installation

### From Source

```bash
git clone https://github.com/sgaunet/logwrap.git
cd logwrap
task build
# Binary will be in ./bin/logwrap
```

### Using Go Install

```bash
go install github.com/sgaunet/logwrap/cmd/logwrap@latest
```

## Quick Start

```bash
# Basic usage
logwrap echo "Hello World"

# With mixed output
logwrap sh -c "echo 'stdout'; echo 'stderr' >&2"

# Using configuration file
logwrap -config examples/basic.yaml make build

# Custom template (timestamp only)
logwrap -template "[{{.Timestamp}}] " ls -la

# Enable colors and UTC time
logwrap -colors -utc make test
```

## Usage

```
logwrap [options] -- <command> [args...]
logwrap [options] <command> [args...]

Options:
  -config string      Configuration file path
  -template string    Log prefix template (default "[{{.Timestamp}}] [{{.Level}}] [{{.User}}:{{.PID}}] ")
  -utc                Use UTC timestamps (default false)
  -colors             Enable colored output (default false)
  -format string      Output format: text, json, structured (default "text")
  -help               Show help message
  -version            Show version information

Note: To control user/PID inclusion, either:
  - Use -template flag to customize the prefix format
  - Edit the config file to set user.enabled or pid.enabled to false
```

## Configuration

LogWrap looks for configuration files in the following order:
1. File specified with `-config` flag
2. `./logwrap.yaml` or `./logwrap.yml`
3. `~/.config/logwrap/config.yaml`
4. `~/.logwrap.yaml`

### Basic Configuration

```yaml
prefix:
  template: "[{{.Timestamp}}] [{{.Level}}] [{{.User}}:{{.PID}}] "
  timestamp:
    # Uses strftime format (Linux date command style)
    # Common: %Y=year %m=month %d=day %H=hour %M=minute %S=second
    format: "%Y-%m-%d %H:%M:%S"
    utc: false
  colors:
    enabled: false
    info: "green"
    error: "red"
    timestamp: "blue"
  user:
    enabled: true      # Control user inclusion in template
    format: "username"  # username, uid, or full
  pid:
    enabled: true      # Control PID inclusion in template
    format: "decimal"   # decimal or hex

output:
  format: "text"        # text, json, or structured
  buffer: "line"        # line, none, or full

log_level:
  default_stdout: "INFO"
  default_stderr: "ERROR"
  detection:
    enabled: true
    keywords:
      error: ["ERROR", "FATAL", "PANIC"]
      warn: ["WARN", "WARNING"]
      debug: ["DEBUG", "TRACE"]
      info: ["INFO"]
```

### Template Variables

- `{{.Timestamp}}` - Formatted timestamp (using strftime format from config)
- `{{.Level}}` - Log level (INFO, ERROR, WARN, DEBUG)
- `{{.User}}` - User information (controlled by user.enabled and user.format in config)
- `{{.PID}}` - Process ID (controlled by pid.enabled and pid.format in config)

### Timestamp Format

LogWrap uses **strftime format** (Linux `date` command style), not Go's time format:

| Directive | Meaning | Example |
|-----------|---------|---------|
| `%Y` | 4-digit year | 2024 |
| `%m` | Month (01-12) | 01 |
| `%d` | Day (01-31) | 15 |
| `%H` | Hour 24h (00-23) | 14 |
| `%M` | Minute (00-59) | 30 |
| `%S` | Second (00-59) | 45 |
| `%z` | Timezone offset | -0700 |
| `%f` | Microseconds | 123456 |
| `%a` | Weekday short | Mon |
| `%b` | Month short | Jan |

**Examples:**
- `%Y-%m-%d %H:%M:%S` → `2024-01-15 14:30:45`
- `%Y-%m-%dT%H:%M:%S%z` → `2024-01-15T14:30:45-0700`
- `%d/%b/%Y %H:%M` → `15/Jan/2024 14:30`

### Color Options

Available colors: `black`, `red`, `green`, `yellow`, `blue`, `magenta`, `cyan`, `white`, `none`

### Log Level Detection

LogWrap automatically detects log levels based on configurable keywords:

- **ERROR**: Lines containing "ERROR", "FATAL", "PANIC"
- **WARN**: Lines containing "WARN", "WARNING"
- **DEBUG**: Lines containing "DEBUG", "TRACE"
- **INFO**: Lines containing "INFO" or default for stdout

## Examples

### Basic Usage

```bash
# Simple command
logwrap echo "Hello World"
# Output: [2024-01-15 10:30:45] [INFO] [user:1234] Hello World

# Command with errors
logwrap sh -c "echo 'Success'; echo 'ERROR: Failed' >&2"
# Output: [2024-01-15 10:30:45] [INFO] [user:1234] Success
#         [2024-01-15 10:30:45] [ERROR] [user:1234] ERROR: Failed
```

### Using Configuration Files

```bash
# Minimal configuration (timestamp only)
logwrap -config examples/minimal.yaml echo "Simple"
# Output: [10:30:45] Simple

# Advanced configuration with UTC and hex PID
logwrap -config examples/advanced.yaml echo "Advanced"
# Output: [2024-01-15T10:30:45.123456+0000] [INFO] [user(1000):0x4d2] Advanced
```

### Custom Templates

```bash
# Timestamp only (no user/PID)
logwrap -template "[{{.Timestamp}}] " echo "Custom"
# Output: [2024-01-15 10:30:45] Custom

# Level and timestamp only
logwrap -template "{{.Level}}: {{.Timestamp}} - " echo "Level first"
# Output: INFO: 2024-01-15 10:30:45 - Level first

# Include user but not PID
logwrap -template "[{{.Level}}] [{{.User}}] " echo "No PID"
# Output: [INFO] [john] No PID
```

### Long-running Commands

```bash
# Monitor a build process
logwrap make build

# Watch log files
logwrap tail -f /var/log/app.log

# Stream processing
logwrap ping google.com
```

## Configuration Examples

See the `examples/` directory for:
- `basic.yaml` - Standard configuration with all features
- `minimal.yaml` - Minimal setup with just timestamps
- `advanced.yaml` - Advanced setup with UTC times and extended keywords
- `test_commands.sh` - Script with various test commands

## Architecture

LogWrap is built with a modular architecture:

- **Config Package**: YAML configuration and CLI flag handling
- **Executor Package**: Command execution with stream capture
- **Processor Package**: Real-time stream processing
- **Formatter Package**: Log formatting and prefix generation with strftime support

### Key Dependencies

- **[github.com/itchyny/timefmt-go](https://github.com/itchyny/timefmt-go)** - Pure Go strftime implementation
  - Provides Linux `date` command compatible timestamp formatting
  - Efficient and standards-compliant

For detailed architecture information, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Development

### Requirements

- Go 1.21 or later
- [Task](https://taskfile.dev/) (task runner)

### Building and Testing

```bash
# Build the binary
task build

# Run all tests
task test

# Run tests with coverage
task test-coverage

# Run tests with race detection
task test-race

# Run linter
task linter

# Create snapshot build
task snapshot
```

For more development commands, see [CLAUDE.md](CLAUDE.md).

## Performance

LogWrap is designed for minimal overhead:
- Real-time processing with no buffering delays
- Efficient memory usage with buffer reuse
- Concurrent processing of stdout/stderr streams
- Minimal CPU impact on wrapped commands

## Troubleshooting

### Common Issues

1. **Command not found**: Ensure the command is in your PATH
2. **Configuration errors**: Validate your YAML syntax
3. **Permission denied**: Check file permissions for config files
4. **Color issues**: Some terminals may not support ANSI colors

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- Create an issue for bug reports or feature requests
- Check existing issues before creating new ones
- Provide detailed information including OS, Go version, and configuration