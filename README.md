# TokenResume

[中文说明](README.zh-CN.md)

`TokenResume` is a watchdog for `Claude/Codex CLI` tasks.  
It continuously monitors target processes, checks token rate-limit status, suspends tasks when quota is exhausted, and resumes them after reset.

## Features

```text
TokenResume/
├── cmd/tokenresume/main.go
├── internal/
│   ├── config/
│   ├── monitor/
│   ├── ratelimit/
│   └── resume/
├── pkg/logger/
├── config.yaml
├── README.md
└── README.zh-CN.md
```

- Supports concurrent monitoring for multiple target processes (e.g. `claude`, `codex`).
- Supports three resume strategies:
  - `sigstop`: resume suspended process via `SIGCONT`
  - `restart`: relaunch task from saved command line
  - `session_replay`: recover task via template command and session info
- Persists task snapshots to local JSON.
- Provides pluggable rate-limit provider abstraction.

## Build

```bash
go mod tidy
go build -o tokenresume ./cmd/tokenresume
```

## Run

```bash
./tokenresume --config config.yaml
```

## Manually Resume a Process

```bash
./tokenresume resume --pid 12345
```

## Configuration

Default config file is `config.yaml`. Key fields:

- `monitor.processes`: regex rules for target process matching.
- `monitor.poll_interval`: process scan interval.
- `monitor.token_check_interval`: token check interval.
- `rate_limit.provider`: rate-limit provider (`anthropic` currently implemented).
- `rate_limit.proxy_endpoint`: optional endpoint for rate-limit proxy.
- `resume.strategy`: resume strategy (`sigstop` / `restart` / `session_replay`).
- `resume.restart_command`: command template used by `session_replay`.
- `resume.snapshot_path`: snapshot file path.

## Runtime Flow (Simplified)

1. Periodically scan target processes and maintain tracked set.
2. Periodically query rate-limit status.
3. When limited:
   - Save snapshots for each target process.
   - Suspend tasks and wait for reset.
4. After reset:
   - Resume tasks by configured strategy.
   - Continue next monitoring loop.

## Notes

- Current implementation is mainly for Linux (`/proc` and Unix signals required).
- `session_replay` depends on external CLI session recovery capability.
- For production hardening, consider adding:
  - Retry/backoff
  - Fine-grained error classification
  - Complete unit and integration tests
