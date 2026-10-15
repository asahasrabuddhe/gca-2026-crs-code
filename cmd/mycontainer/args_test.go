package main

import (
	"strings"
	"testing"
)

func TestParseRun(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantImage string
		wantArgv  string
		wantErr   bool
	}{
		{name: "image and command", args: []string{"ubuntu", "bash"}, wantImage: "ubuntu", wantArgv: "bash"},
		{name: "command with arguments", args: []string{"ubuntu", "/bin/sh", "-c", "echo hi"}, wantImage: "ubuntu", wantArgv: "/bin/sh -c echo hi"},
		{name: "nothing", args: nil, wantErr: true},
		{name: "image but no command", args: []string{"ubuntu"}, wantErr: true},
		{name: "a path is not an image name", args: []string{"rootfs/ubuntu", "bash"}, wantErr: true},
		{name: "no climbing out of rootfs/", args: []string{"..", "bash"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image, argv, err := parseRun(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want error, got %q %v", image, argv)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if image != tt.wantImage || strings.Join(argv, " ") != tt.wantArgv {
				t.Errorf("got %q %q, want %q %q", image, strings.Join(argv, " "), tt.wantImage, tt.wantArgv)
			}
		})
	}
}
