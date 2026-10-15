package container

import (
	"errors"
	"os/exec"
	"syscall"
)

// exitCode turns the error from cmd.Run into the code the runtime exits
// with. A container that exits 5 makes mycontainer exit 5, and one killed
// by signal N exits 128+N, the shell's convention. Any other error means
// the container never ran, and is returned as an error.
func exitCode(err error) (int, error) {
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return 0, err
	}
	if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal()), nil
	}
	return exitErr.ExitCode(), nil
}
