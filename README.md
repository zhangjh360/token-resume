# TokenResume

`TokenResume` 是一个面向 `Claude/Codex CLI` 任务的守护程序。  
它会持续监控目标进程，检测 Token 限流状态，在额度耗尽时自动挂起任务，并在重置后自动恢复。

## 功能特性

- 支持同时监控多个目标进程（如 `claude`、`codex`）。
- 支持三种恢复策略：
  - `sigstop`：通过 `SIGCONT` 恢复已挂起进程
  - `restart`：根据快照中的命令行重新拉起任务
  - `session_replay`：基于模板命令和会话信息恢复任务
- 支持任务快照持久化到本地 JSON 文件。
- 支持限流检测 Provider 抽象，便于扩展不同平台。

## 目录结构

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
└── README.md
```

## 编译

```bash
go mod tidy
go build -o tokenresume ./cmd/tokenresume
```

## 启动

```bash
./tokenresume --config config.yaml
```

## 手动恢复指定进程

```bash
./tokenresume resume --pid 12345
```

## 配置说明

默认配置文件为 `config.yaml`，核心字段如下：

- `monitor.processes`：要监控的进程匹配规则（正则）。
- `monitor.poll_interval`：进程扫描间隔。
- `monitor.token_check_interval`：Token 状态检查间隔。
- `rate_limit.provider`：限流检测提供方（当前已实现 `anthropic`）。
- `rate_limit.proxy_endpoint`：可选的限流代理接口地址。
- `resume.strategy`：恢复策略（`sigstop` / `restart` / `session_replay`）。
- `resume.restart_command`：`session_replay` 模式下的恢复命令模板。
- `resume.snapshot_path`：快照文件保存位置。

## 运行机制（简版）

1. 周期性扫描目标进程并维护跟踪列表。
2. 周期性查询限流状态。
3. 触发限流时：
   - 为每个目标进程保存快照；
   - 按策略挂起任务并等待重置。
4. 到达重置时间后：
   - 按配置策略尝试恢复；
   - 主流程继续进入下一轮监控。

## 注意事项

- 本项目目前主要面向 Linux 环境（依赖 `/proc` 与 Unix 信号）。
- `session_replay` 依赖外部 CLI 的会话恢复能力，请按实际命令调整模板。
- 如需更稳定接入生产环境，建议增加：
  - 重试退避
  - 更细粒度错误分类
  - 完整的单元测试与集成测试
