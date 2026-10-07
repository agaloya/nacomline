package main

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

// A Nacomline project folder:
//
//	nacomline.conf          the only file the user edits
//	input/ output/ ...      Matriline's spool, with its state/ (ledger, key)
//	.nacomline/server.conf  generated: Matriline's server, on 127.0.0.1 only
//	.nacomline/client/      generated: Matriline's client (client.conf, its credential, state)
//
// The server and the client are the unchanged matriline-server and matriline-client next
// to the nacomline program (or in NACOMLINE_MATRILINE), so a new Matriline needs no new
// Nacomline.

type paths struct {
	conf, project, hidden, server, clientDir, client, cred string
}

func pathsOf(c *conf) paths {
	h := filepath.Join(c.Folder, ".nacomline")
	return paths{conf: c.path, project: c.Folder, hidden: h, server: filepath.Join(h, "server.conf"),
		clientDir: filepath.Join(h, "client"), client: filepath.Join(h, "client", "client.conf"),
		cred: filepath.Join(h, "client", "credential.conf")}
}

// matrilineBin finds matriline-server or matriline-client.
func matrilineBin(prog string) (string, error) {
	name := "matriline-" + prog
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	dirs := []string{os.Getenv("NACOMLINE_MATRILINE")}
	if self, err := os.Executable(); err == nil {
		if r, err := filepath.EvalSymlinks(self); err == nil {
			self = r
		}
		dirs = append(dirs, filepath.Dir(self))
	}
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if p := filepath.Join(d, name); fileExists(p) {
			return p, nil
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	return "", errors.New(Tf("%s not found: put it next to nacomline (or set NACOMLINE_MATRILINE to its folder)", name))
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// hiddenCommands are Matriline commands that make no sense on one computer, or that
// Nacomline does itself (MATRILINE_HIDE leaves them out of help, console and web).
const hiddenCommands = "init,run,stop,restart,service,restore,edit,config,update,clients,keys,kit,block,unblock,bans,console,web"

// matriline runs a Matriline program with this project's generated configuration.
func matriline(p paths, prog string, args ...string) *exec.Cmd {
	bin, err := matrilineBin(prog)
	if err != nil {
		bin = "matriline-" + prog // the error shows when it runs
	}
	cfg := p.server
	if prog == "client" {
		cfg = p.client
	}
	cmd := exec.Command(bin, append([]string{"-c", cfg}, args...)...)
	// MATRILINE_PARENT_PID: they stop by themselves if this program is gone (killed, or a
	// terminal 'web' ended); macOS has no Pdeathsig. Matriline 39853f6 or later
	cmd.Env = append(os.Environ(), "MATRILINE_HIDE="+hiddenCommands, "MATRILINE_PROG=nacomline",
		"MATRILINE_PARENT_PID="+strconv.Itoa(os.Getpid()))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd
}

// capture runs a Matriline command and returns its output.
func capture(p paths, prog string, args ...string) (string, error) {
	cmd := matriline(p, prog, args...)
	var out bytes.Buffer
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, &out, &out
	err := cmd.Run()
	return strings.TrimSpace(out.String()), err
}

func cmdInit(dir, asked string) error {
	if asked != "" && !slices.Contains(languages, strings.ToLower(asked)) {
		return fmt.Errorf("language %q: %s", asked, strings.Join(languages, ", "))
	}
	lang := setLang(asked)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	confPath := filepath.Join(dir, confName)
	if fileExists(confPath) {
		return errors.New(Tf("%s already exists", confPath))
	}
	if err := os.WriteFile(confPath, []byte(setValue(confTemplate(lang), "general", "language", lang)), 0o600); err != nil {
		return err
	}
	c, err := loadConf(confPath)
	if err != nil {
		return err
	}
	p := pathsOf(c)
	if fileExists(p.server) {
		return errors.New(Tf("%s already exists", p.server))
	}
	if err := os.MkdirAll(p.hidden, 0o700); err != nil {
		return err
	}
	// Matriline's own init: the spool folders and the server key in the project folder,
	// the documented server.conf moved into the hidden folder; the client's next to it.
	srvBin, err := matrilineBin("server")
	if err != nil {
		return err
	}
	cliBin, err := matrilineBin("client")
	if err != nil {
		return err
	}
	initSrv := exec.Command(srvBin, "init", p.project, "--language", lang)
	initSrv.Env = append(os.Environ(), "MATRILINE_NO_REGISTRY=1") // its own projects stay off Matriline's list
	if out, err := initSrv.CombinedOutput(); err != nil {
		return fmt.Errorf("matriline-server init: %v: %s", err, out)
	}
	if err := os.Rename(filepath.Join(p.project, "server.conf"), p.server); err != nil {
		return err
	}
	initCli := exec.Command(cliBin, "init", p.clientDir, "--language", lang)
	initCli.Env = append(os.Environ(), "MATRILINE_NO_REGISTRY=1")
	if out, err := initCli.CombinedOutput(); err != nil {
		return fmt.Errorf("matriline-client init: %v: %s", err, out)
	}
	// the ORCA folder Matriline found goes into nacomline.conf
	cli, _ := os.ReadFile(p.client)
	orca := strings.TrimSpace(strings.Split(getValue(string(cli), "orca", "paths"), ",")[0])
	text, _ := os.ReadFile(confPath)
	if err := os.WriteFile(confPath, []byte(setValue(string(text), "orca", "path", orca)), 0o600); err != nil {
		return err
	}
	// the fixed settings of a single computer, once; the user's ones at every apply
	port, err := freePort()
	if err != nil {
		return err
	}
	if err := editFile(p.server, fixedServer(p, port)); err != nil {
		return err
	}
	if err := editFile(p.client, fixedClient); err != nil {
		return err
	}
	c, err = loadConf(confPath)
	if err != nil {
		return err
	}
	if _, err := apply(c); err != nil {
		return err
	}
	fmt.Println(Tf("created %s", confPath))
	if orca == "" {
		fmt.Println(T("ORCA was not found: set its folder in nacomline.conf ([orca] path)"))
	} else {
		fmt.Printf("ORCA: %s\n", orca)
	}
	fmt.Println(Tf("next: put inputs (*.inp) in %s, then run 'nacomline -c %s run'\n  (or 'nacomline -c %s service install' to start it by itself)",
		filepath.Join(p.project, "input"), confPath, confPath))
	return nil
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

type kv struct{ section, key, value string }

// fixedServer: one computer, nobody to distrust. The output checks stay (ORCA errors,
// unconverged optimizations, inconsistent geometries go to weird/); what needs other
// computers or guards against them is off. Matriline's active re-checks need another
// computer unless verify.same_host: nacomline.conf's [checks] verify (apply).
func fixedServer(p paths, port int) []kv {
	addr := "127.0.0.1:" + strconv.Itoa(port)
	return []kv{
		{"spool", "root", p.project},
		{"network", "listen", addr},
		{"network", "advertise", addr},
		{"network", "connection", "direct"},
		{"network", "enrollment", "issued"},
		{"verify", "enabled", "true"},
		{"verify", "output_consistency", "true"},
		{"verify", "fingerprints", "true"},
		{"verify", "timing_plausibility", "false"},
		{"verify", "canary_rate", "0"},
		{"verify", "replication_rate", "0"},
		{"verify", "probation_replication", "0"},
		{"verify", "second_opinion", "false"},
		{"verify", "reputation", "false"},
		{"verify", "retry_weird", "false"},
		{"verify", "rescue_weird", "false"},
		{"tasks", "orca_error_hosts", "1"},
		{"update", "mode", "off"},
	}
}

var fixedClient = []kv{
	{"security", "updates", "false"}, // Nacomline is updated as a whole
	{"security", "sandbox", "true"},
}

func editFile(path string, sets []kv) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(b)
	for _, s := range sets {
		text = setValue(text, s.section, s.key, s.value)
	}
	return writeIfChanged(path, text)
}

func writeIfChanged(path, text string) error {
	if old, err := os.ReadFile(path); err == nil && string(old) == text {
		return nil
	}
	tmp := path + ".new"
	if err := os.WriteFile(tmp, []byte(text), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// apply writes the user's settings from nacomline.conf into the generated files. It
// reports whether the client's file changed (the client reads it when it starts).
func apply(c *conf) (clientChanged bool, err error) {
	p := pathsOf(c)
	if !fileExists(p.server) || !fileExists(p.client) {
		return false, errors.New(T("this folder was not set up by 'nacomline init' (no .nacomline/)"))
	}
	memTotal := "0"
	if !strings.EqualFold(c.Memory, "auto") {
		memTotal = c.Memory
	}
	frac := "0"
	if c.Verify {
		frac = "0.05" // Matriline's defaults
	}
	if err := editFile(p.server, []kv{
		{"verify", "scf_recheck_fraction", frac},
		{"verify", "gradient_check_fraction", frac},
		{"verify", "hessian_probe_fraction", frac},
		{"verify", "same_host", strconv.FormatBool(c.Verify)},
		{"verify", "quarantine_after", "0"}, // one computer: nothing to quarantine
		{"orca", "reference_paths", c.OrcaPath},
		{"alerts", "command", c.AlertCmd},
		{"log", "level", c.LogLevel},
		{"general", "language", c.Language},
	}); err != nil {
		return false, err
	}
	before, _ := os.ReadFile(p.client)
	err = editFile(p.client, []kv{
		{"general", "language", c.Language},
		{"orca", "paths", c.OrcaPath},
		{"resources", "cores", strconv.Itoa(c.Cores)},
		{"resources", "memory_total", memTotal},
		{"schedule", "windows", c.Schedule},
		{"schedule", "pause_on_battery", strconv.FormatBool(c.OnBattery)},
		{"limits", "cpu_temperature_limit", strconv.FormatFloat(c.MaxTempC, 'f', -1, 64)},
	})
	after, _ := os.ReadFile(p.client)
	return !bytes.Equal(before, after), err
}
