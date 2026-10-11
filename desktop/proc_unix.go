//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// own process group, so a terminal Ctrl-C to the shell doesn't race our graceful stop
func setProcAttrs(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
