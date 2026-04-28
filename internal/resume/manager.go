package resume

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"tokenresume/internal/config"
	"tokenresume/pkg/logger"
)

type TaskSnapshot struct {
	PID         int       `json:"pid"`
	Command     string    `json:"command"`
	Args        []string  `json:"args"`
	WorkingDir  string    `json:"working_dir"`
	CommandLine string    `json:"command_line"`
	StdinHistory []string `json:"stdin_history"`
	LastOutput  string    `json:"last_output"`
	SessionID   string    `json:"session_id"`
	SavedAt     time.Time `json:"saved_at"`
}

type Manager struct {
	cfg       config.ResumeConfig
	log       *logger.Logger
	snapshots map[int]*TaskSnapshot
	mu        sync.RWMutex
}

func NewManager(cfg config.ResumeConfig, log *logger.Logger) *Manager {
	m := &Manager{
		cfg:       cfg,
		log:       log,
		snapshots: make(map[int]*TaskSnapshot),
	}
	_ = m.loadSnapshotFile()
	return m
}

func (m *Manager) Strategy() string {
	return m.cfg.Strategy
}

func (m *Manager) SaveSnapshot(snapshot *TaskSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshots[snapshot.PID] = snapshot
	return m.saveSnapshotFile()
}

func (m *Manager) ListSnapshots() []*TaskSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*TaskSnapshot, 0, len(m.snapshots))
	for _, s := range m.snapshots {
		out = append(out, s)
	}
	return out
}

func (m *Manager) Resume(ctx context.Context, snapshot *TaskSnapshot) error {
	strategy, fallback, err := m.pickStrategies()
	if err != nil {
		return err
	}

	if err := strategy.Resume(ctx, snapshot); err == nil {
		return nil
	}
	return fallback.Resume(ctx, snapshot)
}

func (m *Manager) pickStrategies() (ResumeStrategy, ResumeStrategy, error) {
	main, err := m.strategyByName(m.cfg.Strategy)
	if err != nil {
		return nil, nil, err
	}
	var fallback ResumeStrategy = &RestartStrategy{}
	if main.Name() == fallback.Name() {
		fallback = &SigcontStrategy{}
	}
	return main, fallback, nil
}

func (m *Manager) strategyByName(name string) (ResumeStrategy, error) {
	switch strings.ToLower(name) {
	case "sigstop", "sigcont":
		return &SigcontStrategy{}, nil
	case "restart":
		return &RestartStrategy{}, nil
	case "session_replay":
		return NewSessionReplayStrategy(m.cfg.RestartCommand), nil
	default:
		return nil, errors.New("unknown resume strategy: " + name)
	}
}

func (m *Manager) loadSnapshotFile() error {
	data, err := os.ReadFile(m.cfg.SnapshotPath)
	if err != nil {
		return nil
	}
	var list []*TaskSnapshot
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	for _, s := range list {
		m.snapshots[s.PID] = s
	}
	return nil
}

func (m *Manager) saveSnapshotFile() error {
	list := make([]*TaskSnapshot, 0, len(m.snapshots))
	for _, s := range m.snapshots {
		list = append(list, s)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.cfg.SnapshotPath, data, 0o644)
}
