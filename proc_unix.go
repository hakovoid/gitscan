//go:build unix

package main

import (
	"os/exec"
	"syscall"
)

// detach lance la commande dans une nouvelle session, sans terminal de contrôle.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
