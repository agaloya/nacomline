//go:build !windows

package main

import "os/exec"

// keepWithParent: nothing after the start on Unix (Linux: Pdeathsig in withParent).
func keepWithParent(cmd *exec.Cmd) {}
