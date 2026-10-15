package main

import (
	"strings"
	"testing"

	"go.ajitem.com/gca-crs/internal/cgroup"
)

func TestParseRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantImage  string
		wantArgv   string
		wantLimits cgroup.Limits
		wantErr    bool
	}{
		{name: "image and command", args: []string{"ubuntu", "bash"}, wantImage: "ubuntu", wantArgv: "bash"},
		{name: "command with arguments", args: []string{"ubuntu", "/bin/sh", "-c", "echo hi"}, wantImage: "ubuntu", wantArgv: "/bin/sh -c echo hi"},
		{
			name:       "limits",
			args:       []string{"-memory", "64m", "-cpus", "0.5", "-pids", "20", "ubuntu", "bash"},
			wantImage:  "ubuntu",
			wantArgv:   "bash",
			wantLimits: cgroup.Limits{Memory: 64 << 20, CPUs: 0.5, Pids: 20},
		},
		{name: "flags after the image belong to the command", args: []string{"ubuntu", "ls", "-pids"}, wantImage: "ubuntu", wantArgv: "ls -pids"},
		{name: "nothing", args: nil, wantErr: true},
		{name: "image but no command", args: []string{"ubuntu"}, wantErr: true},
		{name: "a path is not an image name", args: []string{"rootfs/ubuntu", "bash"}, wantErr: true},
		{name: "no climbing out of rootfs/", args: []string{"..", "bash"}, wantErr: true},
		{name: "bad memory", args: []string{"-memory", "lots", "ubuntu", "bash"}, wantErr: true},
		{name: "negative pids", args: []string{"-pids", "-1", "ubuntu", "bash"}, wantErr: true},
		{name: "unknown flag", args: []string{"-bogus", "ubuntu", "bash"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, err := parseRun(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want error, got %+v", o)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if o.image != tt.wantImage || strings.Join(o.argv, " ") != tt.wantArgv {
				t.Errorf("got %q %q, want %q %q", o.image, strings.Join(o.argv, " "), tt.wantImage, tt.wantArgv)
			}
			if o.limits != tt.wantLimits {
				t.Errorf("limits = %+v, want %+v", o.limits, tt.wantLimits)
			}
		})
	}
}
