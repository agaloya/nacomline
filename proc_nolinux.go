//go:build !linux && !windows

package main

import "os/exec"

func withParent(cmd *exec.Cmd) {}
