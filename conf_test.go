package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConf(t *testing.T) {
	dir := t.TempDir()
	c, err := parseConf(filepath.Join(dir, confName), defaultConf)
	if err != nil {
		t.Fatal(err)
	}
	if c.Folder != dir || c.Cores != 0 || c.Memory != "auto" || !c.OnBattery || c.LogLevel != "info" {
		t.Errorf("defaults: %+v", c)
	}
	bad := strings.Replace(defaultConf, "cores = 0", "cores = many", 1) + "\n[x]\ny = 1\n"
	if _, err := parseConf(filepath.Join(dir, confName), bad); err == nil || !strings.Contains(err.Error(), "computer.cores") || !strings.Contains(err.Error(), "unknown setting x.y") {
		t.Errorf("bad file: %v", err)
	}
	rel := strings.Replace(defaultConf, "folder = .", "folder = data", 1)
	if c, _ := parseConf(filepath.Join(dir, confName), rel); c.Folder != filepath.Join(dir, "data") {
		t.Errorf("relative folder: %s", c.Folder)
	}
}

func TestSetValue(t *testing.T) {
	text := "[a]\nx = 1\n\n[b]\ny = 2 # note\n\n[a]\nz = 3\n"
	text = setValue(text, "a", "z", "9")      // in the second part of [a]
	text = setValue(text, "b", "y", "7")      // replaced, comment dropped
	text = setValue(text, "b", "new", "yes")  // added to [b]
	text = setValue(text, "c", "k", "v")      // a new section
	text = setValue(text, "a", "x", "a b, c") // spaces kept
	for k, want := range map[string]string{"a.x": "a b, c", "a.z": "9", "b.y": "7", "b.new": "yes", "c.k": "v"} {
		vals, _ := parseINI(text)
		if vals[k] != want {
			t.Errorf("%s = %q, want %q\n%s", k, vals[k], want, text)
		}
	}
	if strings.Count(text, "z =") != 1 {
		t.Errorf("duplicated key:\n%s", text)
	}
}

// apply writes the user's settings into Matriline's generated files.
func TestApply(t *testing.T) {
	dir := t.TempDir()
	conf := strings.NewReplacer("path =", "path = /opt/orca", "cores = 0", "cores = 6", "memory = auto", "memory = 24GiB",
		"schedule =", "schedule = mon-fri 20:00-07:00", "level = info", "level = debug").Replace(defaultConf)
	os.WriteFile(filepath.Join(dir, confName), []byte(conf), 0o600)
	c, err := loadConf(filepath.Join(dir, confName))
	if err != nil {
		t.Fatal(err)
	}
	p := pathsOf(c)
	os.MkdirAll(p.clientDir, 0o700)
	os.WriteFile(p.server, []byte("[orca]\nreference_paths = /x\n\n[alerts]\ncommand =\n\n[log]\nlevel = info\n"), 0o600)
	os.WriteFile(p.client, []byte("[orca]\npaths = /x\n\n[resources]\ncores = 0\nmemory_per_core = 1GiB\n\n[schedule]\nwindows =\npause_on_battery = true\n\n[limits]\ncpu_temperature_limit = 0\n"), 0o600)
	changed, err := apply(c)
	if err != nil || !changed {
		t.Fatalf("apply: %v, changed %v", err, changed)
	}
	srv, _ := os.ReadFile(p.server)
	cli, _ := os.ReadFile(p.client)
	for _, want := range []string{"reference_paths = /opt/orca", "level = debug"} {
		if !strings.Contains(string(srv), want) {
			t.Errorf("server.conf lacks %q", want)
		}
	}
	for _, want := range []string{"paths = /opt/orca", "cores = 6", "memory_total = 24GiB", "windows = mon-fri 20:00-07:00"} {
		if !strings.Contains(string(cli), want) {
			t.Errorf("client.conf lacks %q:\n%s", want, cli)
		}
	}
	if changed, _ := apply(c); changed {
		t.Error("a second apply changed the client's file")
	}
}

// Values with or without quotes mean the same (user).
func TestQuotedValues(t *testing.T) {
	dir := t.TempDir()
	text := strings.Replace(defaultConf, "level = info", `level = "warn"`, 1)
	text = strings.Replace(text, "memory = auto", "memory = 'auto'", 1)
	c, err := parseConf(filepath.Join(dir, confName), text)
	if err != nil {
		t.Fatal(err)
	}
	if c.LogLevel != "warn" || c.Memory != "auto" {
		t.Errorf("quoted values: level %q memory %q", c.LogLevel, c.Memory)
	}
}
