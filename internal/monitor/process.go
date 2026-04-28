package monitor

import (
	"bytes"
	"fmt"
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
