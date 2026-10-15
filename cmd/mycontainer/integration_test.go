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
	"syscall"
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

// runAsUser executes mycontainer as the user who ran sudo, with no
// privileges at all. From 03-user that is how the talk runs it.
func runAsUser(t *testing.T, args ...string) (result, int) {
	t.Helper()
	uid, err1 := strconv.Atoi(os.Getenv("SUDO_UID"))
	gid, err2 := strconv.Atoi(os.Getenv("SUDO_GID"))
	if err1 != nil || err2 != nil || uid == 0 {
		t.Skip("run through sudo by an ordinary user: make test-root")
	}
	cmd := exec.Command(binary, args...)
	cmd.Dir = repo
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}}
	return wait(t, cmd), uid
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

// ---------------------------------------------------------------- 02-mnt ---

func TestTheContainerHasItsOwnProc(t *testing.T) {
	// A mount namespace, a fresh /proc mounted from inside the PID
	// namespace, and /proc lists the container: the shell, ls and wc.
	if n := count(t, run(t, sh("ls -d /proc/[0-9]* | wc -l")...).stdout); n > 5 {
		t.Errorf("/proc lists %d processes; want only the container's own", n)
	}
}

func TestTheHostFilesystemIsGone(t *testing.T) {
	marker := filepath.Join("/tmp", "crs-host-marker-"+strconv.Itoa(os.Getpid()))
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(marker)

	r := run(t, sh("test -e "+marker+" && echo HOST; test -e /.old_root && echo OLDROOT; cat /etc/os-release")...)
	if strings.Contains(r.stdout, "HOST") {
		t.Error("a file on the host's /tmp is visible inside")
	}
	if strings.Contains(r.stdout, "OLDROOT") {
		t.Error("the old root's parking directory survived the pivot")
	}
	if !strings.Contains(r.stdout, "Ubuntu") {
		t.Errorf("the rootfs is not the ubuntu image:\n%s", r.stdout)
	}
}

func TestAMissingImageIsAnError(t *testing.T) {
	r := run(t, "run", "no-such-image", "/bin/true")
	if r.code == 0 || !strings.Contains(r.stderr, "rootfs") {
		t.Errorf("want a rootfs error, got code %d:\n%s", r.code, r.stderr)
	}
}

// --------------------------------------------------------------- 03-user ---

func TestRootInsideTheUserOutside(t *testing.T) {
	r, uid := runAsUser(t, sh("id -u; cat /proc/self/uid_map")...)
	if r.code != 0 {
		t.Fatalf("no sudo, no container: code %d\n%s", r.code, r.stderr)
	}
	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if len(lines) != 2 || lines[0] != "0" {
		t.Fatalf("want uid 0 and a one-line map, got:\n%s", r.stdout)
	}
	if got := strings.Fields(lines[1]); len(got) != 3 || got[0] != "0" || got[1] != strconv.Itoa(uid) || got[2] != "1" {
		t.Errorf("uid_map = %q, want 0 %d 1", lines[1], uid)
	}
}

func TestUnderSudoRootMapsToTheImagesOwner(t *testing.T) {
	// The image was pulled by the user, so it belongs to them. Container
	// root maps to that owner, which is what lets root inside write to it.
	var st syscall.Stat_t
	if err := syscall.Stat(filepath.Join(repo, "rootfs", "ubuntu"), &st); err != nil {
		t.Fatal(err)
	}
	r := run(t, sh("cat /proc/self/uid_map; touch /root/written && echo WROTE")...)
	if f := strings.Fields(r.stdout); len(f) < 3 || f[1] != strconv.Itoa(int(st.Uid)) {
		t.Errorf("uid_map does not map to the image's owner %d:\n%s", st.Uid, r.stdout)
	}
	if !strings.Contains(r.stdout, "WROTE") {
		t.Errorf("root inside could not write to its own image:\n%s%s", r.stdout, r.stderr)
	}
	_ = os.Remove(filepath.Join(repo, "rootfs", "ubuntu", "root", "written"))
}

// ---------------------------------------------------------------- 04-uts ---

func TestTheHostnameIsTheContainerID(t *testing.T) {
	before, _ := os.Hostname()
	r, _ := runAsUser(t, sh("hostname")...)
	if !regexp.MustCompile(`^[0-9a-f]{12}$`).MatchString(strings.TrimSpace(r.stdout)) {
		t.Errorf("hostname inside = %q, want the twelve-character container id\n%s", r.stdout, r.stderr)
	}
	if after, _ := os.Hostname(); after != before {
		t.Fatalf("the host was renamed from %q to %q", before, after)
	}
}

// ---------------------------------------------------------------- 05-ipc ---

func TestTheHostsSharedMemoryIsInvisible(t *testing.T) {
	// The 2025 demo: make a segment on the host, then look for it inside.
	out, err := exec.Command("ipcmk", "-M", "1024").Output()
	if err != nil {
		t.Fatalf("ipcmk: %v", err)
	}
	m := regexp.MustCompile(`id: (\d+)`).FindStringSubmatch(string(out))
	if m == nil {
		t.Fatalf("ipcmk said %q", out)
	}
	shmid := m[1]
	defer exec.Command("ipcrm", "-m", shmid).Run()

	r, _ := runAsUser(t, sh("ipcs -m")...)
	if !strings.Contains(r.stdout, "Shared Memory Segments") {
		t.Fatalf("ipcs did not run:\n%s%s", r.stdout, r.stderr)
	}
	for _, line := range strings.Split(r.stdout, "\n") {
		if f := strings.Fields(line); len(f) > 1 && f[1] == shmid {
			t.Errorf("the host's segment %s is visible inside:\n%s", shmid, r.stdout)
		}
	}
}

// ------------------------------------------------------------ 06-cgroups ---

func TestLimitsAreVisibleInside(t *testing.T) {
	r := run(t, "run", "-memory", "64m", "-cpus", "0.5", "-pids", "20", "ubuntu", "/bin/sh", "-c",
		"cat /proc/self/cgroup /sys/fs/cgroup/memory.max /sys/fs/cgroup/cpu.max /sys/fs/cgroup/pids.max")
	// 0::/ is the cgroup namespace: the container's cgroup is its root.
	for _, want := range []string{"0::/\n", "67108864\n", "50000 100000\n", "20\n"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("missing %q in:\n%s%s", want, r.stdout, r.stderr)
		}
	}
}

func TestAForkBombIsRefused(t *testing.T) {
	r := run(t, "run", "-pids", "20", "ubuntu", "/bin/sh", "-c", "for i in $(seq 40); do sleep 2 & done; wait")
	if !strings.Contains(r.stderr, "Cannot fork") {
		t.Errorf("fork was never refused:\n%s", r.stderr)
	}
}

func TestAMemoryHogIsKilled(t *testing.T) {
	// A 100 MB string in a 64 MB cgroup. The OOM killer takes the shell,
	// and a death by SIGKILL is exit 137.
	r := run(t, "run", "-memory", "64m", "ubuntu", "/bin/sh", "-c", "a=$(head -c 100000000 /dev/zero | tr '\\0' x); echo survived")
	if strings.Contains(r.stdout, "survived") || r.code != 137 {
		t.Errorf("the hog survived, code %d:\n%s", r.code, r.stdout)
	}
}

func TestTheCgroupIsRemovedAfterExit(t *testing.T) {
	sweep()
	run(t, "run", "-pids", "20", "ubuntu", "/bin/true")
	entries, err := os.ReadDir("/sys/fs/cgroup/mycontainer")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			t.Errorf("leftover cgroup %s", e.Name())
		}
	}
}

// sweep removes cgroups an earlier, killed run left behind, so the test
// judges only the container it runs.
func sweep() {
	entries, _ := os.ReadDir("/sys/fs/cgroup/mycontainer")
	for _, e := range entries {
		if e.IsDir() {
			_ = os.Remove(filepath.Join("/sys/fs/cgroup/mycontainer", e.Name()))
		}
	}
}
