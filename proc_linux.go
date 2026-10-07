package main

import (
	"os/exec"
	"syscall"
)

// withParent: the Matriline processes end with Nacomline even when it is killed outright
// (otherwise the server would keep the project locked and the next start would fail).
func withParent(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
}
