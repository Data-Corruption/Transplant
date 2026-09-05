package transplant

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func configureProcess(cmd *exec.Cmd) {
	// cut/build launch Go and shell children. Cancel the whole group so an
	// interrupted wizard cannot leave a finalizer editing files behind it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}
