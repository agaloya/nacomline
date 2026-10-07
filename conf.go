package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// nacomline.conf is the only configuration the user sees. Matriline's server.conf and
// client.conf are generated from it into .nacomline/ (see setup.go); everything that only
// matters between several computers (keys, bans, quarantine, the network) is fixed there.

const confName = "nacomline.conf"

const defaultConf = `# Nacomline configuration. After a change: 'nacomline config apply' (the web page's
# settings do it when you save). Lines starting with '#' are comments.

[general]
# en: language of Nacomline's messages, help, menus and web page: "en", "es", "fr", "pt", "ar" (empty: this computer's)
# es: idioma de los mensajes, la ayuda, los menús y la página web de Nacomline: "en", "es", "fr", "pt", "ar" (vacío: el de esta computadora)
# fr: langue des messages, de l'aide, des menus et de la page web de Nacomline : "en", "es", "fr", "pt", "ar" (vide : celle de cet ordinateur)
# pt: idioma das mensagens, da ajuda, dos menus e da página web do Nacomline: "en", "es", "fr", "pt", "ar" (vazio: o deste computador)
# ar: لغة رسائل Nacomline ومساعدته وقوائمه وصفحة الويب: "en", "es", "fr", "pt", "ar" (فارغ: لغة هذا الحاسوب؛ الترجمة العربية آلية ولم تُراجَع بعد)
language =

[project]
# folder with input/ output/ weird/ errors/ ...; "." = this file's folder
folder = .

[orca]
# ORCA installation (init finds it)
path =

[computer]
# cores ORCA may use; 0 = all physical cores
cores = 0
# memory ORCA may use in total (e.g. 16GiB); "auto" = what the computer can spare
memory = auto
# when new calculations start (running ones always finish); empty = always.
# e.g.: mon-fri 20:00-07:00, sat-sun 00:00-24:00
schedule =
# stop taking new calculations while on battery
pause_on_battery = true
# stop taking new calculations above this CPU temperature (Celsius); 0 = no limit
max_cpu_temperature = 0

[checks]
# ORCA's output is always checked (errors, unconverged optimizations, inconsistent
# geometries go to weird/ or errors/). verify = "yes" also recomputes a few results in
# part (SCF energy, gradient, Hessian) on this same computer: no use against cheating
# here, but it can reveal a faulty machine (bad memory, overheating) that changes a
# result without ORCA noticing. It costs some extra computing time.
verify = no

[alerts]
# program run for each alert (event, message), e.g. a Telegram script; empty = none
command =

[log]
# "debug", "info", "warn" or "error"
level = info
`

// conf is nacomline.conf as read.
type conf struct {
	path      string // the file
	Folder    string // absolute project folder
	OrcaPath  string
	Cores     int
	Memory    string // "auto" or a size
	Schedule  string
	OnBattery bool
	MaxTempC  float64
	AlertCmd  string
	Verify    bool
	LogLevel  string
	Language  string // general.language: en, es, fr, pt, ar; empty = this computer's
}

// parseINI reads "[section]" and "key = value" lines; '#' starts a comment (as in
// Matriline's files).
func parseINI(text string) (map[string]string, error) {
	vals := map[string]string{}
	sec := ""
	for i, line := range strings.Split(text, "\n") {
		if j := strings.Index(line, "#"); j >= 0 {
			line = line[:j]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sec = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || sec == "" {
			return nil, fmt.Errorf("line %d: expected key = value inside a [section]", i+1)
		}
		v = strings.TrimSpace(v)
		// key = value, key = "value" and key = 'value' mean the same (user)
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
			v = v[1 : len(v)-1]
		}
		vals[sec+"."+strings.TrimSpace(k)] = v
	}
	return vals, nil
}

var sizeRe = regexp.MustCompile(`^(?i)\d+(\.\d+)?\s*(k|m|g|t)(i?b)?$`)

// loadConf reads and checks nacomline.conf.
func loadConf(path string) (*conf, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseConf(path, string(b))
}

func parseConf(path, text string) (*conf, error) {
	vals, err := parseINI(text)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", filepath.Base(path), err)
	}
	known := map[string]bool{"project.folder": true, "orca.path": true, "computer.cores": true, "computer.memory": true,
		"computer.schedule": true, "computer.pause_on_battery": true, "computer.max_cpu_temperature": true,
		"alerts.command": true, "log.level": true, "checks.verify": true, "general.language": true}
	var errs []string
	for k := range vals {
		if !known[k] {
			errs = append(errs, T("unknown setting")+" "+k)
		}
	}
	abs, _ := filepath.Abs(path)
	c := &conf{path: abs, Memory: "auto", OnBattery: true, LogLevel: "info"}
	dir := filepath.Dir(abs)
	c.Folder = dir
	if f := vals["project.folder"]; f != "" {
		if !filepath.IsAbs(f) {
			f = filepath.Join(dir, f)
		}
		c.Folder = filepath.Clean(f)
	}
	c.OrcaPath = vals["orca.path"]
	if v := vals["computer.cores"]; v != "" {
		if c.Cores, err = strconv.Atoi(v); err != nil || c.Cores < 0 {
			errs = append(errs, T("computer.cores: a whole number, 0 = all"))
		}
	}
	if v := vals["computer.memory"]; v != "" && !strings.EqualFold(v, "auto") {
		if !sizeRe.MatchString(v) {
			errs = append(errs, T("computer.memory: auto or a size such as 16GiB"))
		}
		c.Memory = v
	}
	c.Schedule = vals["computer.schedule"]
	if v := vals["computer.pause_on_battery"]; v != "" {
		if c.OnBattery, err = strconv.ParseBool(v); err != nil {
			errs = append(errs, T("computer.pause_on_battery: true or false"))
		}
	}
	if v := vals["computer.max_cpu_temperature"]; v != "" {
		if c.MaxTempC, err = strconv.ParseFloat(v, 64); err != nil || c.MaxTempC < 0 {
			errs = append(errs, T("computer.max_cpu_temperature: degrees Celsius, 0 = no limit"))
		}
	}
	if v := strings.ToLower(vals["checks.verify"]); v != "" {
		switch v {
		case "yes", "true", "on":
			c.Verify = true
		case "no", "false", "off":
		default:
			errs = append(errs, T("checks.verify: yes or no"))
		}
	}
	c.Language = strings.ToLower(vals["general.language"])
	switch c.Language {
	case "", "en", "es", "fr", "pt", "ar":
	default:
		errs = append(errs, T("general.language: en, es, fr, pt, ar or empty (this computer's)"))
	}
	c.AlertCmd = vals["alerts.command"]
	if c.AlertCmd != "" && !filepath.IsAbs(c.AlertCmd) {
		errs = append(errs, T("alerts.command: an absolute path"))
	}
	if v := vals["log.level"]; v != "" {
		c.LogLevel = strings.ToLower(v)
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, T("log.level: debug, info, warn or error"))
	}
	if len(errs) > 0 {
		return c, errors.New(filepath.Base(path) + ": " + strings.Join(errs, "; "))
	}
	return c, nil
}

// warnings are not errors, but worth saying (config check, web settings).
func (c *conf) warnings() []string {
	var w []string
	if c.OrcaPath == "" {
		w = append(w, T("orca.path is empty: set the ORCA installation folder"))
	} else if _, err := os.Stat(c.OrcaPath); err != nil {
		w = append(w, "orca.path: "+err.Error())
	}
	if _, err := os.Stat(c.Folder); err != nil {
		w = append(w, "project.folder: "+err.Error())
	}
	return w
}

// setValue sets key in [section] of an INI text: the first "key =" line of that section
// (a section may appear in several parts of the file), otherwise a new line at the end of
// the section's first part. Comments after the value are dropped with it.
func setValue(text, section, key, value string) string {
	lines := strings.Split(text, "\n")
	cur, first := "", -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			cur = strings.TrimSpace(t[1 : len(t)-1])
			if cur == section && first < 0 {
				first = i
			}
			continue
		}
		if cur != section || strings.HasPrefix(t, "#") {
			continue
		}
		if k, _, ok := strings.Cut(t, "="); ok && strings.TrimSpace(k) == key {
			lines[i] = key + " = " + value
			return strings.Join(lines, "\n")
		}
	}
	line := key + " = " + value
	if first < 0 {
		return strings.TrimRight(text, "\n") + "\n\n[" + section + "]\n" + line + "\n"
	}
	end := len(lines)
	for i := first + 1; i < len(lines); i++ {
		if t := strings.TrimSpace(lines[i]); strings.HasPrefix(t, "[") {
			end = i
			break
		}
	}
	for end > first+1 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	out := append([]string{}, lines[:end]...)
	out = append(out, line)
	return strings.Join(append(out, lines[end:]...), "\n")
}

// getValue reads key in [section] of an INI text.
func getValue(text, section, key string) string {
	vals, _ := parseINI(text)
	return vals[section+"."+key]
}
