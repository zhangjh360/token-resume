package main

import "testing"

func TestParseResumeCommand(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantHandle bool
		wantPID    int
		wantErr    bool
	}{
		{
			name:       "non resume command",
			args:       []string{"--config", "config.yaml"},
			wantHandle: false,
		},
		{
			name:       "resume with pid",
			args:       []string{"resume", "--pid", "123"},
			wantHandle: true,
			wantPID:    123,
			wantErr:    false,
		},
		{
			name:       "resume missing pid",
			args:       []string{"resume"},
			wantHandle: true,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handle, pid, err := parseResumeCommand(tt.args)
			if handle != tt.wantHandle {
				t.Fatalf("handle mismatch: got %v want %v", handle, tt.wantHandle)
			}
			if pid != tt.wantPID {
				t.Fatalf("pid mismatch: got %d want %d", pid, tt.wantPID)
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("error mismatch: got %v wantErr %v", err, tt.wantErr)
			}
		})
	}
}
