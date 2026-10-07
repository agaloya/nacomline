package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// 'nacomline service install|remove': 'nacomline run' starts by itself (as Matriline's
// client does it: a systemd user unit on Linux, a Task Scheduler task on Windows, a launchd
// agent on macOS in service_launchd.go).

func serviceUnit(confPath string) string {
	sum := sha256.Sum256([]byte(confPath))
	return "nacomline-" + hex.EncodeToString(sum[:4])
}

func cmdService(c *conf, args []string) error {
	if len(args) == 0 || (args[0] != "install" && args[0] != "remove") {
		return errors.New(T("usage: service install | service remove"))
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if r, err := filepath.EvalSymlinks(self); err == nil {
		self = r
	}
	switch runtime.GOOS {
	case "linux":
		return serviceSystemd(args[0], self, c.path)
	case "windows":
		return serviceWindows(args[0], self, c.path)
	case "darwin":
		return serviceLaunchd(c, args[0], self)
	}
	return errors.New(Tf("not available on %s yet: start it with '%s -c %s run'", runtime.GOOS, self, c.path))
}

func runCmd(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %v: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func serviceSystemd(action, self, confPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	name := serviceUnit(confPath)
	unit := filepath.Join(home, ".config", "systemd", "user", name+".service")
	if action == "remove" {
		runCmd("systemctl", "--user", "disable", "--now", name)
		if err := os.Remove(unit); err != nil && !os.IsNotExist(err) {
			return err
		}
		runCmd("systemctl", "--user", "daemon-reload")
		fmt.Println(T("removed the service; Nacomline no longer starts by itself"))
		return nil
	}
	text := fmt.Sprintf(`[Unit]
Description=Nacomline (ORCA calculations on this computer)
After=network.target

[Service]
WorkingDirectory=%s
ExecStart=%q -c %q run
Restart=on-failure
RestartSec=30
KillMode=mixed
TimeoutStopSec=90

[Install]
WantedBy=default.target
`, filepath.Dir(confPath), self, confPath)
	if err := os.MkdirAll(filepath.Dir(unit), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(unit, []byte(text), 0o644); err != nil {
		return err
	}
	if err := runCmd("systemctl", "--user", "daemon-reload"); err != nil {
		return err
	}
	if err := runCmd("systemctl", "--user", "enable", "--now", name); err != nil {
		return err
	}
	fmt.Println(Tf("installed and started %s (systemd user service %s)", unit, name))
	fmt.Println(T("it starts when you log in; to keep it running while you are logged out: loginctl enable-linger $USER"))
	fmt.Println(Tf("stop it: %s   remove it: %s", "systemctl --user stop "+name, "nacomline service remove"))
	return nil
}

func serviceWindows(action, self, confPath string) error {
	task := serviceUnit(confPath)
	if action == "remove" {
		runCmd("schtasks", "/End", "/TN", task)
		if err := runCmd("schtasks", "/Delete", "/TN", task, "/F"); err != nil {
			return err
		}
		fmt.Println(T("removed the scheduled task; Nacomline no longer starts by itself"))
		return nil
	}
	u, err := user.Current()
	if err != nil {
		return err
	}
	when := T("when the computer starts (nobody needs to log in)")
	tr := fmt.Sprintf(`"%s" -c "%s" run`, self, confPath)
	if err := runCmd("schtasks", "/Create", "/TN", task, "/TR", tr, "/SC", "ONSTART", "/RU", u.Username, "/NP", "/RL", "LIMITED", "/F"); err != nil {
		fmt.Println(Tf("note: a task at start-up needs an administrator (%v);\n  this one starts when you log in. For start-up, run this command again in an administrator PowerShell.", err))
		when = T("when you log in")
		tr = fmt.Sprintf(`conhost.exe --headless "%s" -c "%s" run`, self, confPath)
		if err := runCmd("schtasks", "/Create", "/TN", task, "/TR", tr, "/SC", "ONLOGON", "/RL", "LIMITED", "/F"); err != nil {
			return err
		}
	}
	if err := runCmd("schtasks", "/Run", "/TN", task); err != nil {
		return err
	}
	fmt.Println(Tf("installed and started the scheduled task %q: Nacomline starts by itself %s", task, when))
	fmt.Println(Tf("stop it: %s   remove it: %s", `schtasks /End /TN "`+task+`"`, "nacomline service remove"))
	return nil
}
