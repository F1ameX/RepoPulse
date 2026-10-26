//go:build !linux && !darwin

package git

import "os/exec"

func isolateProcess(cmd *exec.Cmd) {}
