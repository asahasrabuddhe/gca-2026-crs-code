//go:build linux

// Package container is the runtime: a parent that starts a child in new
// namespaces, and the child that becomes the container.
//
// They have to be two processes. Namespaces are a property of a process,
// and the flags that create them are passed to clone(2), which makes a new
// one. A Go program cannot move itself into a new PID namespace after the
// fact: the runtime is already several threads by the time main runs, and
// a new PID namespace only applies to children anyway. So the parent
// starts its own binary again, through /proc/self/exe, with the clone
// flags set, and that child is the container. runc does the same and calls
// it runc init.
package container

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"go.ajitem.com/gca-crs/internal/rootfs"
)

// ChildCommand is the argv[1] the parent starts its own binary with.
const ChildCommand = "child"

// Run is the parent. It returns the container's exit code.
func Run(image string, argv []string) (int, error) {
	logPID("run")

	root, err := rootfsPath(image)
	if err != nil {
		return 0, err
	}
	// Fail here, with a clear message, rather than as an EINVAL from
	// pivot_root inside the child.
	if err := rootfs.Validate(root); err != nil {
		return 0, err
	}
	uid, gid, err := owner(root)
	if err != nil {
		return 0, err
	}

	// GidMappingsEnableSetgroups stays false, its default: the kernel
	// refuses an unprivileged gid_map unless setgroups is denied first,
	// and os/exec denies it when the field is false.
	// snippet:start clone
	id := newID()
	cmd := exec.Command("/proc/self/exe", append([]string{ChildCommand, id, root}, argv...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// 01-pid: its own process IDs. 02-mnt: its own mount table.
		// 03-user: its own user IDs, so no sudo. 04-uts: its own hostname.
		// 05-ipc: its own shared memory, semaphores and message queues.
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUSER |
			syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC,
		// One line each: root inside is the image's owner outside.
		UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: uid, Size: 1}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: gid, Size: 1}},
	}
	// snippet:end clone

	return exitCode(cmd.Run())
}

// Child runs inside the new namespaces as PID 1. args is the container
// ID, the rootfs path and the command.
func Child(args []string) error {
	logPID("child")
	if len(args) < 3 {
		return errors.New("child: want an id, a rootfs and a command")
	}
	id, root, argv := args[0], args[1], args[2:]

	// snippet:start hostname
	// Only this container sees it, because of CLONE_NEWUTS.
	if err := syscall.Sethostname([]byte(id)); err != nil {
		return fmt.Errorf("sethostname: %w", err)
	}
	// snippet:end hostname

	// snippet:start child-pivot
	// The image becomes /, with its own /proc, /dev and /sys, and the
	// host's filesystem is detached. Only possible because of CLONE_NEWNS:
	// without it, this would do the same to the host.
	if err := rootfs.Pivot(rootfs.Linux{}, root); err != nil {
		return err
	}
	// snippet:end child-pivot

	// exec, not fork: the command replaces this process and so inherits
	// PID 1, with everything that will mean later.
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return err
	}
	return syscall.Exec(path, argv, os.Environ())
}

// owner is the user and group that own the image. Container root maps to
// them. Run as yourself, that is you, and the map is one line no setuid
// helper is needed for. Run with sudo, it is still the image's owner,
// because root inside has to be able to write to the image, and a
// capability inside a user namespace means nothing to a file whose owner
// is not mapped into it.
func owner(root string) (uid, gid int, err error) {
	var st syscall.Stat_t
	if err := syscall.Stat(root, &st); err != nil {
		return 0, 0, err
	}
	if os.Geteuid() != 0 && int(st.Uid) != os.Geteuid() {
		return 0, 0, fmt.Errorf("%s belongs to uid %d, not you; pull it as yourself (make rootfs)", root, st.Uid)
	}
	return int(st.Uid), int(st.Gid), nil
}

// rootfsPath is rootfs/<image> under the working directory, the same
// place the 2025 version looked.
func rootfsPath(image string) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "rootfs", image), nil
}

// logPID prints which side is talking and what PID it has. The child says
// "as pid 1", which is the whole point of 01-pid.
func logPID(side string) {
	fmt.Fprintf(os.Stderr, "[%s] as pid %d\n", side, os.Getpid())
}
