// Nacomline - ORCA quantum-chemistry jobs on one computer, by driving the unchanged
// Matriline server and client (https://github.com/agaloya/matriline) on this computer only.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const version = "0.1.0"

const usage = `nacomline - ORCA quantum-chemistry jobs on one computer

Usage: nacomline [-c nacomline.conf] <command>

  init [dir] [--language code]
                      set up a project folder: nacomline.conf, input/ output/ ... (finds
                      ORCA); --language: en, es, fr, pt or ar (default: this computer's)
  run                 compute: takes the inputs in input/, results go to output/, weird/,
                      errors/ (normally run by the service)
  service install|remove
                      start by itself (Linux: systemd user service; Windows: scheduled task;
                      macOS: launchd agent)
  status              what is queued, running and done; whether the computer is paused
  watch               live view: the calculations running now, refreshed every 2 s
  web                 the same on a local web page, with the settings
  console             every command from numbered menus
  pause [duration] [--now]
                      stop using this computer for a while (e.g. "pause 2h"; running
                      calculations finish, or stop at once with --now); resume: use it again
  pause <input/...>   hold inputs back (resume <paused/...> releases them)
  config show|check|apply|edit
                      nacomline.conf: print it, check it, apply it (after editing it by
                      hand), or edit it in $EDITOR and apply it
  doctor              check ORCA, the sandbox and the setup
  version
  Every other Matriline server command works too: add, review, accept, reject, redo,
  retry, history, stats, events, check, verify, backup, live ... ('nacomline help-all')
`

func main() {
	args := os.Args[1:]
	confPath := os.Getenv("NACOMLINE_CONF")
	if confPath == "" {
		confPath = confName
	}
	if len(args) >= 2 && (args[0] == "-c" || args[0] == "--config") {
		confPath, args = args[1], args[2:]
	}
	// the language: nacomline.conf's, else this computer's (read before anything prints)
	if c, err := loadConf(confPath); err == nil {
		setLang(c.Language)
	} else {
		setLang("")
	}
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, T(usage))
		os.Exit(2)
	}
	if err := dispatch(confPath, args); err != nil {
		fmt.Fprintln(os.Stderr, T("error:"), err)
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			os.Exit(ee.ExitCode())
		}
		os.Exit(1)
	}
}

func dispatch(confPath string, args []string) error {
	switch args[0] {
	case "help", "-h", "--help":
		if len(args) > 1 {
			if t := commandUsage(args[1]); t != "" {
				fmt.Print(t)
				return nil
			}
			// a Matriline command: its own detailed help
			if bin, err := matrilineBin("server"); err == nil {
				cmd := exec.Command(bin, append([]string{"help"}, args[1:]...)...)
				cmd.Env = append(os.Environ(), "MATRILINE_PROG=nacomline")
				cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
				return cmd.Run()
			}
		}
		fmt.Print(T(usage))
		return nil
	case "version":
		fmt.Printf("nacomline %s\n", version)
		for _, prog := range []string{"server", "client"} {
			if bin, err := matrilineBin(prog); err == nil {
				out, _ := exec.Command(bin, "version", "--plain").Output()
				fmt.Printf("  %s", out)
			}
		}
		return nil
	case "init":
		dir, lang := ".", ""
		for i := 1; i < len(args); i++ {
			switch {
			case args[i] == "--language" && i+1 < len(args):
				lang = args[i+1]
				i++
			case strings.HasPrefix(args[i], "--language="):
				lang = strings.TrimPrefix(args[i], "--language=")
			default:
				dir = args[i]
			}
		}
		return cmdInit(dir, lang)
	}
	c, err := loadConf(confPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New(Tf("%s not found: run 'nacomline init <folder>' first, or give -c <folder>/nacomline.conf", confPath))
		}
		return err
	}
	p := pathsOf(c)
	switch args[0] {
	case "run":
		return cmdRun(c)
	case "service":
		return cmdService(c, args[1:])
	case "status":
		out, err := capture(p, "server", append([]string{"status"}, args[1:]...)...)
		if err != nil && strings.Contains(out, "not running") {
			fmt.Println(T("Nacomline is not running, or it is starting (its first start checks ORCA: a minute or two).\nStart it with 'nacomline run' or 'nacomline service install'."))
			return err
		}
		fmt.Println(out)
		if out, _ := capture(p, "client", "status"); out != "" {
			for _, l := range strings.Split(out, "\n") {
				if strings.Contains(l, "paused") || strings.HasPrefix(l, "service:") && !strings.Contains(l, "running, connected") {
					fmt.Println(T("this computer:") + " " + l)
				}
			}
		}
		return err
	case "pause", "resume":
		if len(args) > 1 && (strings.Contains(args[1], "/") || args[1] == "all") {
			return matriline(p, "server", args...).Run() // inputs
		}
		return matriline(p, "client", args...).Run() // this computer
	case "doctor":
		e1 := matriline(p, "server", "doctor").Run()
		e2 := matriline(p, "client", "doctor").Run()
		return errors.Join(e1, e2)
	case "watch", "console":
		return matriline(p, "server", args...).Run()
	case "web":
		self, err := os.Executable()
		if err != nil {
			return err
		}
		cmd := matriline(p, "server", args...)
		cmd.Env = append(cmd.Env, "MATRILINE_WEB_CONF="+p.conf, "MATRILINE_WEB_APPLY="+self, "NACOMLINE_CONF="+p.conf)
		return cmd.Run()
	case "config":
		return cmdConfig(c, args[1:])
	case "web-config-check": // the web page's settings (MATRILINE_WEB_APPLY)
		if len(args) < 2 {
			return errors.New("usage: web-config-check <file>")
		}
		b, err := os.ReadFile(args[1])
		if err != nil {
			return err
		}
		cand, err := parseConf(c.path, string(b))
		if err != nil {
			return err
		}
		for _, w := range cand.warnings() {
			fmt.Println(T("warning:") + " " + w)
		}
		return nil
	case "web-config-apply":
		return cmdConfig(c, []string{"apply"})
	case "help-all":
		return matriline(p, "server", "help").Run()
	}
	return matriline(p, "server", args...).Run()
}

func cmdConfig(c *conf, a []string) error {
	p := pathsOf(c)
	sub := "show"
	if len(a) > 0 {
		sub = a[0]
	}
	switch sub {
	case "show":
		b, err := os.ReadFile(p.conf)
		fmt.Print(string(b))
		return err
	case "check":
		for _, w := range c.warnings() {
			fmt.Println(T("warning:") + " " + w)
		}
		fmt.Println(T("nacomline.conf is valid"))
		return nil
	case "apply":
		changed, err := apply(c)
		if err != nil {
			return err
		}
		if out, err := capture(p, "server", "config", "reload"); err == nil {
			fmt.Println(T("applied to the running server:") + " " + lastLine(out))
		} else {
			fmt.Println(T("saved; it applies when Nacomline runs"))
		}
		if changed {
			fmt.Println(T("this computer's settings changed: the running Nacomline restarts its client within seconds (calculations continue from their last checkpoint)"))
		}
		return nil
	case "edit":
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = defaultEditor()
		}
		cmd := exec.Command(editor, p.conf)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
		c2, err := loadConf(p.conf)
		if err != nil {
			return errors.New(Tf("%v (nothing applied; fix it and run 'nacomline config apply')", err))
		}
		return cmdConfig(c2, []string{"apply"})
	}
	return errors.New(T("usage: config show|check|apply|edit"))
}

// commandUsage is the part of the usage text about one command ('help <command>').
func commandUsage(cmd string) string {
	var b strings.Builder
	in := false
	for _, l := range strings.Split(T(usage), "\n") {
		if strings.HasPrefix(l, "  ") && !strings.HasPrefix(l, "   ") {
			f := strings.Fields(l)
			in = len(f) > 0 && f[0] == cmd
		}
		if in {
			b.WriteString(l + "\n")
		}
	}
	return b.String()
}

func lastLine(s string) string {
	l := strings.Split(strings.TrimSpace(s), "\n")
	return l[len(l)-1]
}

func defaultEditor() string {
	if filepath.Separator == '\\' {
		return "notepad"
	}
	return "nano"
}
