//go:build windows

package main

import (
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

// On Windows a child outlives its parent, and a task started at boot has no console for
// Matriline's programs to watch: Nacomline puts them in one Job Object that is closed with
// "kill on close", so they end with Nacomline whatever ends it (service remove, task
// stopped, crash). Matriline's client nests its own job per calculation inside it.

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW   = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJob  = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJob = kernel32.NewProc("AssignProcessToJobObject")
	jobOnce                sync.Once
	job                    syscall.Handle
)

type jobBasicLimit struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type jobExtendedLimit struct {
	Basic                 jobBasicLimit
	IoInfo                [6]uint64
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

func withParent(cmd *exec.Cmd) {}

// keepWithParent puts a started child in Nacomline's job object.
func keepWithParent(cmd *exec.Cmd) {
	jobOnce.Do(func() {
		h, _, _ := procCreateJobObjectW.Call(0, 0)
		if h == 0 {
			return
		}
		var info jobExtendedLimit
		info.Basic.LimitFlags = 0x2000 // JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		if r, _, _ := procSetInformationJob.Call(h, 9, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info)); r == 0 {
			syscall.CloseHandle(syscall.Handle(h))
			return
		}
		job = syscall.Handle(h) // kept open until Nacomline ends
	})
	if job == 0 || cmd.Process == nil {
		return
	}
	const access = 0x0100 | 0x0001 // PROCESS_SET_QUOTA | PROCESS_TERMINATE
	ph, err := syscall.OpenProcess(access, false, uint32(cmd.Process.Pid))
	if err != nil {
		return
	}
	defer syscall.CloseHandle(ph)
	procAssignProcessToJob.Call(uintptr(job), uintptr(ph))
}
