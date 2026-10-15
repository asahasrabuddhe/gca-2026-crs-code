//go:build linux && integration

// End-to-end tests: real containers, as root, inside the VM.
//
//	make test-root
//
// Each test pins one thing the talk shows at this tag, so a demo that stops
// working fails here before it fails in front of a room.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var repo, binary string

func TestMain(m *testing.M) {
	if os.Geteuid() != 0 {
		println("integration tests need root: make test-root")
		os.Exit(1)
	}
	var err error
	if repo, err = filepath.Abs("../.."); err != nil {
		panic(err)
	}
	binary = filepath.Join(repo, "mycontainer")
	for _, p := range []string{binary, filepath.Join(repo, "rootfs", "ubuntu", "bin", "bash")} {
		if _, err := os.Stat(p); err != nil {
			println("missing", p, "- run make build rootfs first")
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}

type result struct {
	stdout, stderr string
	code           int
}

// run executes mycontainer from the repository root, where rootfs/ lives.
func run(t *testing.T, args ...string) result {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = repo
	return wait(t, cmd)
}

func wait(t *testing.T, cmd *exec.Cmd) result {
	t.Helper()
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	r := result{stdout: out.String(), stderr: errb.String()}
	if exitErr, ok := err.(*exec.ExitError); ok {
		r.code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("%v: %v\n%s", cmd.Args, err, errb.String())
	}
	return r
}

// sh is `run ubuntu /bin/sh -c script`.
func sh(script string) []string {
	return []string{"run", "ubuntu", "/bin/sh", "-c", script}
}

func count(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		t.Fatalf("not a count: %q", s)
	}
	return n
}

// ---------------------------------------------------------------- 01-pid ---

func TestBothSidesLogTheirPID(t *testing.T) {
	r := run(t, sh("true")...)
	if !regexp.MustCompile(`(?m)^\[run\] as pid \d+$`).MatchString(r.stderr) {
		t.Errorf("no [run] line:\n%s", r.stderr)
	}
	if !strings.Contains(r.stderr, "[child] as pid 1\n") {
		t.Errorf("the child is not PID 1:\n%s", r.stderr)
	}
}

func TestTheCommandIsPID1(t *testing.T) {
	// The child execs the command, so the command inherits PID 1.
	if r := run(t, sh("echo $$")...); strings.TrimSpace(r.stdout) != "1" {
		t.Errorf("$$ = %q, want 1\n%s", r.stdout, r.stderr)
	}
}

func TestExitCodeComesBack(t *testing.T) {
	if r := run(t, sh("exit 5")...); r.code != 5 {
		t.Errorf("exit code %d, want 5", r.code)
	}
}

func TestAPIDNamespaceAloneStillSeesTheHostsProc(t *testing.T) {
	// The gotcha at this tag: new PIDs, but /proc is still the host's, so
	// anything that reads it, ps included, sees every process on the box.
	if n := count(t, run(t, sh("ls -d /proc/[0-9]* | wc -l")...).stdout); n < 10 {
		t.Errorf("/proc lists %d processes; without a mount namespace it should be the host's many", n)
	}
}
