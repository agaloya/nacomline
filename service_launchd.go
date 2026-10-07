package main

import (
	"errors"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// macOS: a per-user LaunchAgent (~/Library/LaunchAgents/<label>.plist, launchd.plist(5)), as
// Matriline's client does it (agaloya/matriline#8). It starts 'nacomline run' when the user
// logs in and again 30 s after a failure (KeepAlive with SuccessfulExit = false, like the
// systemd unit's Restart=on-failure); stopped normally (SIGTERM) it stays stopped until the
// next login. The label holds the hash of the configuration's path, one agent per project.

func launchdLabel(confPath string) string { return "org.matriline." + serviceUnit(confPath) }

// launchdPlist is the LaunchAgent's property list. ExitTimeOut gives 'nacomline run' the
// time to stop Matriline's server and client cleanly, like TimeoutStopSec of the unit.
func launchdPlist(label, self, confPath, logPath string) string {
	x := html.EscapeString // XML text: & < > " '
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + x(label) + `</string>
	<key>ProgramArguments</key>
	<array>
		<string>` + x(self) + `</string>
		<string>-c</string>
		<string>` + x(confPath) + `</string>
		<string>run</string>
	</array>
	<key>WorkingDirectory</key>
	<string>` + x(filepath.Dir(confPath)) + `</string>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
	<key>ThrottleInterval</key>
	<integer>30</integer>
	<key>ExitTimeOut</key>
	<integer>90</integer>
	<key>StandardOutPath</key>
	<string>` + x(logPath) + `</string>
	<key>StandardErrorPath</key>
	<string>` + x(logPath) + `</string>
</dict>
</plist>
`
}

// launchdDomain is where the agent is loaded: the login session (gui/<uid>) when there is
// one, otherwise the user's background domain (user/<uid>, e.g. over SSH).
func launchdDomain() string {
	uid := strconv.Itoa(os.Getuid())
	if exec.Command("launchctl", "print", "gui/"+uid).Run() == nil {
		return "gui/" + uid
	}
	return "user/" + uid
}

func serviceLaunchd(c *conf, action, self string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	p := pathsOf(c)
	label := launchdLabel(c.path)
	plist := filepath.Join(home, "Library", "LaunchAgents", label+".plist")
	domain := launchdDomain()
	if action == "remove" {
		if _, err := os.Stat(plist); os.IsNotExist(err) {
			fmt.Println(Tf("no service is installed for %s", c.path))
			return nil
		}
		runCmd("launchctl", "bootout", domain+"/"+label) // stops Nacomline (SIGTERM)
		if err := os.Remove(plist); err != nil && !os.IsNotExist(err) {
			return err
		}
		fmt.Println(T("removed the service; Nacomline no longer starts by itself"))
		return nil
	}
	// a 'nacomline run' already running for this project (from a terminal) holds Matriline's
	// server: the agent's copy would fail at once and launchd would start it again every 30 s
	if _, err := capture(p, "server", "status"); err == nil {
		return errors.New(Tf("Nacomline is already running for %s: stop it first (Ctrl+C in its terminal), then install the service", c.path))
	}
	if err := os.MkdirAll(filepath.Dir(plist), 0o755); err != nil {
		return err
	}
	logPath := filepath.Join(p.hidden, "service.log")
	if err := os.WriteFile(plist, []byte(launchdPlist(label, self, c.path, logPath)), 0o644); err != nil {
		return err
	}
	runCmd("launchctl", "bootout", domain+"/"+label) // an older version of this agent
	if err := runCmd("launchctl", "bootstrap", domain, plist); err != nil {
		if strings.HasPrefix(domain, "user/") {
			return fmt.Errorf("%v (log in to this Mac once, or run the command in its Terminal)", err)
		}
		return err
	}
	fmt.Println(Tf("installed and started %s (launchd agent %s)", plist, label))
	fmt.Println(Tf("it starts when you log in; its output goes to %s", logPath))
	fmt.Println(Tf("stop it: %s   remove it: %s", "launchctl bootout "+domain+"/"+label, "nacomline service remove"))
	return nil
}
