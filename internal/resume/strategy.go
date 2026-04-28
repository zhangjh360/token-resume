package resume

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"text/template"
)

type ResumeStrategy interface {
	Name() string
	Resume(ctx context.Context, snapshot *TaskSnapshot) error
}

type SigcontStrategy struct{}

func (s *SigcontStrategy) Name() string {
	return "sigstop"
}

func (s *SigcontStrategy) Resume(ctx context.Context, snapshot *TaskSnapshot) error {
	return syscall.Kill(snapshot.PID, syscall.SIGCONT)
}

type RestartStrategy struct{}

func (s *RestartStrategy) Name() string {
	return "restart"
}

func (s *RestartStrategy) Resume(ctx context.Context, snapshot *TaskSnapshot) error {
	cmd, err := commandFromLine(snapshot.CommandLine)
	if err != nil {
		return err
	}
	if snapshot.WorkingDir != "" {
		cmd.Dir = snapshot.WorkingDir
	}
	return cmd.Start()
}

type SessionReplayStrategy struct {
	commandTemplate string
}

func NewSessionReplayStrategy(tpl string) *SessionReplayStrategy {
	return &SessionReplayStrategy{commandTemplate: tpl}
}

func (s *SessionReplayStrategy) Name() string {
	return "session_replay"
}

func (s *SessionReplayStrategy) Resume(ctx context.Context, snapshot *TaskSnapshot) error {
	if strings.TrimSpace(s.commandTemplate) == "" {
		return errors.New("empty restart command template")
	}
	content, err := renderCommandTemplate(s.commandTemplate, snapshot)
	if err != nil {
		return err
	}
	cmd, err := commandFromLine(content)
	if err != nil {
		return err
	}
	if snapshot.WorkingDir != "" {
		cmd.Dir = snapshot.WorkingDir
	}
	return cmd.Start()
}

func commandFromLine(commandLine string) (*exec.Cmd, error) {
	parts := strings.Fields(commandLine)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command line")
	}
	return exec.Command(parts[0], parts[1:]...), nil
}

func renderCommandTemplate(tpl string, snapshot *TaskSnapshot) (string, error) {
	t, err := template.New("restart").Parse(tpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, snapshot); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}
