package container

import (
	"errors"
	"os/exec"
	"testing"
)

func TestExitCode(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		want    int
		wantErr bool
	}{
		{name: "success", err: nil, want: 0},
		{name: "exit 3", err: exec.Command("sh", "-c", "exit 3").Run(), want: 3},
		// A shell convention the container keeps: death by signal N is 128+N.
		{name: "killed by SIGTERM", err: exec.Command("sh", "-c", "kill -TERM $$").Run(), want: 143},
		{name: "not an exit at all", err: errors.New("fork/exec: no such file"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := exitCode(tt.err)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want error, got code %d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}
