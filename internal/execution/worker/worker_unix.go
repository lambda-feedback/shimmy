//go:build aix || darwin || dragonfly || freebsd || (js && wasm) || linux || nacl || netbsd || openbsd || solaris

package worker

import (
	"os/exec"
	"syscall"
)

func (p *ProcessWorker) killProcess(force bool) error {
	signal := syscall.SIGTERM
	if force {
		signal = syscall.SIGKILL
	}
	if pgid, err := syscall.Getpgid(p.cmd.Process.Pid); err == nil {
		// Negative pid sends signal to all in process group
		return syscall.Kill(-pgid, signal)
	} else {
		return syscall.Kill(p.cmd.Process.Pid, signal)
	}
}

func initCmd(cmd *exec.Cmd) {
	// Start the worker in its own session (which also gives it its own process
	// group, so killProcess can still signal the whole group via -pgid). A new
	// session — rather than just Setpgid — is required for nsjail's own
	// setsid() in --mode e to succeed: setsid() returns EPERM when the caller
	// is already a process-group leader, which Setpgid alone would make it.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}
