package monitor

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"tokenresume/internal/config"
	"tokenresume/internal/resume"
)

type ProcessInfo struct {
	PID        int
	Command    string
	Args       []string
	StartTime  time.Time
	WorkingDir string
	IsRunning  bool
}

type ProcessMonitor struct {
	patterns []targetPattern
}

type targetPattern struct {
	name string
	re   *regexp.Regexp
}

type RateLimitHint struct {
	IsLimited bool
	ResetAt   time.Time
	Source    string
}

func NewProcessMonitor(patterns []config.ProcessPattern) (*ProcessMonitor, error) {
	out := make([]targetPattern, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid process pattern %s: %w", p.Pattern, err)
		}
		out = append(out, targetPattern{name: p.Name, re: re})
	}
	return &ProcessMonitor{patterns: out}, nil
}

func (m *ProcessMonitor) ListTargetProcesses() ([]ProcessInfo, error) {
	cmd := exec.Command("ps", "-eo", "pid,lstart,args")
	raw, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(raw), "\n")
	results := make([]ProcessInfo, 0)
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		startRaw := strings.Join(fields[1:6], " ")
		startAt, _ := time.Parse("Mon Jan 2 15:04:05 2006", startRaw)
		argLine := strings.Join(fields[6:], " ")
		if !m.match(argLine) {
			continue
		}
		argFields := strings.Fields(argLine)
		name := ""
		if len(argFields) > 0 {
			name = argFields[0]
		}
		wd, _ := readWorkingDir(pid)
		results = append(results, ProcessInfo{
			PID:        pid,
			Command:    name,
			Args:       argFields[1:],
			StartTime:  startAt,
			WorkingDir: wd,
			IsRunning:  true,
		})
	}
	return results, nil
}

func (m *ProcessMonitor) BuildSnapshot(p ProcessInfo) (*resume.TaskSnapshot, error) {
	sessionID, _ := detectSessionID(p)
	commandLine := strings.TrimSpace(strings.Join(append([]string{p.Command}, p.Args...), " "))
	return &resume.TaskSnapshot{
		PID:         p.PID,
		Command:     p.Command,
		Args:        p.Args,
		WorkingDir:  p.WorkingDir,
		CommandLine: commandLine,
		SessionID:   sessionID,
		SavedAt:     time.Now(),
	}, nil
}

func (m *ProcessMonitor) match(args string) bool {
	for _, p := range m.patterns {
		if p.re.MatchString(args) {
			return true
		}
	}
	return false
}

func SuspendProcess(pid int) error {
	return syscall.Kill(pid, syscall.SIGSTOP)
}

func ResumeProcess(pid int) error {
	return syscall.Kill(pid, syscall.SIGCONT)
}

func TerminateProcess(pid int) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}

func (m *ProcessMonitor) SuspendProcess(pid int) error {
	return SuspendProcess(pid)
}

func (m *ProcessMonitor) ResumeProcess(pid int) error {
	return ResumeProcess(pid)
}

func (m *ProcessMonitor) TerminateProcess(pid int) error {
	return TerminateProcess(pid)
}

func readWorkingDir(pid int) (string, error) {
	link := filepath.Join("/proc", strconv.Itoa(pid), "cwd")
	return os.Readlink(link)
}

func detectSessionID(p ProcessInfo) (string, error) {
	full := strings.Join(append([]string{p.Command}, p.Args...), " ")
	candidates := []string{"--resume", "--session", "--session-id"}
	for _, token := range candidates {
		idx := strings.Index(full, token)
		if idx < 0 {
			continue
		}
		rest := strings.TrimSpace(full[idx+len(token):])
		if strings.HasPrefix(rest, "=") {
			rest = strings.TrimSpace(rest[1:])
		}
		parts := strings.Fields(rest)
		if len(parts) > 0 {
			return strings.Trim(parts[0], "\"'"), nil
		}
	}
	return "", nil
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil
}

func buildCommand(commandLine string) *exec.Cmd {
	parts := strings.Fields(commandLine)
	if len(parts) == 0 {
		return nil
	}
	return exec.Command(parts[0], parts[1:]...)
}

func runDetached(commandLine, wd string) error {
	cmd := buildCommand(commandLine)
	if cmd == nil {
		return fmt.Errorf("empty command line")
	}
	if wd != "" {
		cmd.Dir = wd
	}
	cmd.Stdout = bytes.NewBuffer(nil)
	cmd.Stderr = bytes.NewBuffer(nil)
	return cmd.Start()
}

func (m *ProcessMonitor) DetectRateLimitHint(terminalsDir, claudeProjectsDir string) (*RateLimitHint, error) {
	best := &RateLimitHint{}
	if hint, err := detectFromTerminalFiles(terminalsDir); err != nil {
		return nil, err
	} else if hint != nil && (best.ResetAt.IsZero() || hint.ResetAt.After(best.ResetAt)) {
		best = hint
	}
	if hint, err := detectFromClaudeProjectLogs(claudeProjectsDir); err != nil {
		return nil, err
	} else if hint != nil && (best.ResetAt.IsZero() || hint.ResetAt.After(best.ResetAt)) {
		best = hint
	}
	if best.ResetAt.IsZero() {
		return nil, nil
	}
	return best, nil
}

func detectFromTerminalFiles(terminalsDir string) (*RateLimitHint, error) {
	if strings.TrimSpace(terminalsDir) == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(terminalsDir)
	if err != nil {
		return nil, err
	}
	best := &RateLimitHint{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		p := filepath.Join(terminalsDir, entry.Name())
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		hint := parseRateLimitHint(string(raw))
		if hint == nil || hint.ResetAt.IsZero() {
			continue
		}
		if best.ResetAt.IsZero() || hint.ResetAt.After(best.ResetAt) {
			best = hint
			best.Source = p
		}
	}
	if best.ResetAt.IsZero() {
		return nil, nil
	}
	return best, nil
}

func detectFromClaudeProjectLogs(projectsDir string) (*RateLimitHint, error) {
	if strings.TrimSpace(projectsDir) == "" {
		return nil, nil
	}
	best := &RateLimitHint{}
	err := filepath.WalkDir(projectsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		raw, err := tailReadFile(path, 64*1024)
		if err != nil {
			return nil
		}
		hint := parseRateLimitHint(raw)
		if hint == nil || hint.ResetAt.IsZero() {
			return nil
		}
		if best.ResetAt.IsZero() || hint.ResetAt.After(best.ResetAt) {
			best = hint
			best.Source = path
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if best.ResetAt.IsZero() {
		return nil, nil
	}
	return best, nil
}

var cnResetPattern = regexp.MustCompile(`限额将在\s*([0-9]{4}-[0-9]{2}-[0-9]{2}\s+[0-9]{2}:[0-9]{2}:[0-9]{2})\s*重置`)
var enResetPattern = regexp.MustCompile(`reset(?:s)?(?:\s+at|\s+on|\s+time)?[:\s]+([0-9]{4}-[0-9]{2}-[0-9]{2}[ T][0-9]{2}:[0-9]{2}:[0-9]{2})`)

func parseRateLimitHint(content string) *RateLimitHint {
	if !strings.Contains(content, "429") &&
		!strings.Contains(content, "已达到 5 小时的使用上限") &&
		!strings.Contains(strings.ToLower(content), "rate limit") {
		return nil
	}
	if m := cnResetPattern.FindStringSubmatch(content); len(m) == 2 {
		resetAt, err := time.ParseInLocation("2006-01-02 15:04:05", m[1], time.Local)
		if err == nil {
			return &RateLimitHint{IsLimited: true, ResetAt: resetAt}
		}
	}
	if m := enResetPattern.FindStringSubmatch(strings.ToLower(content)); len(m) == 2 {
		layout := "2006-01-02 15:04:05"
		val := strings.ReplaceAll(m[1], "t", " ")
		resetAt, err := time.ParseInLocation(layout, val, time.Local)
		if err == nil {
			return &RateLimitHint{IsLimited: true, ResetAt: resetAt}
		}
	}
	return nil
}

func tailReadFile(path string, maxBytes int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return "", err
	}
	size := stat.Size()
	offset := int64(0)
	if size > maxBytes {
		offset = size - maxBytes
	}
	buf := make([]byte, size-offset)
	if _, err := f.ReadAt(buf, offset); err != nil {
		return "", err
	}
	return string(buf), nil
}
