//go:build !linux

package transplant

import "os/exec"

// The wizard refuses non-Linux hosts before starting any subprocesses.
func configureProcess(cmd *exec.Cmd) {}
