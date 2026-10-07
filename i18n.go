package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"os/exec"
	"slices"
	"strings"
)

// Translations of what people read (help, messages, nacomline.conf), in the same format
// as Matriline's (common/i18n): lang/<code>.txt holds "en: <English>" / "<code>: <text>"
// pairs separated by blank lines, a text of several lines continuing on lines that start
// with "  |"; lang/nacomline.conf.<code> is the whole template, its first line
// "# source <sum>" of the English one. Made from lang/strings.txt (go run ./i18nextract).
// A text without a translation stays in English. The menus, help and web page of the
// Matriline commands come translated from Matriline itself (general.language).

//go:embed lang
var langFS embed.FS

var tr map[string]string // English -> the chosen language

// languages are the ones Nacomline and Matriline offer.
var languages = []string{"en", "es", "fr", "pt", "ar"}

// detectLang is the language to use: nacomline.conf's, else the one Matriline detects on
// this computer (matriline-server detect-language: LANGUAGE, LANG, display language,
// keyboards), else English.
func detectLang(configured string) string {
	l := strings.ToLower(strings.TrimSpace(configured))
	if l == "" {
		if bin, err := matrilineBin("server"); err == nil {
			if out, err := exec.Command(bin, "detect-language").Output(); err == nil {
				l = strings.TrimSpace(string(out))
			}
		}
	}
	if i := strings.IndexAny(l, "_-.@"); i >= 0 {
		l = l[:i]
	}
	if !slices.Contains(languages, l) {
		return "en"
	}
	return l
}

// setLang uses the translation of detectLang(configured), if Nacomline has one, and
// returns the language code (which Matriline's programs are given even without one).
func setLang(configured string) string {
	l := detectLang(configured)
	tr = nil
	b, err := langFS.ReadFile("lang/" + l + ".txt")
	if l == "en" || err != nil {
		return l
	}
	es, err := parseCatalog(string(b), l)
	if err != nil {
		return l
	}
	tr = map[string]string{}
	for _, e := range es {
		if e.tr != "" {
			tr[e.en] = e.tr
		}
	}
	return l
}

// T translates a text; Tf translates a format and fills it like fmt.Sprintf.
func T(s string) string {
	if t, ok := tr[s]; ok {
		return t
	}
	return s
}

func Tf(format string, a ...any) string { return fmt.Sprintf(T(format), a...) }

type entry struct{ en, tr string }

func parseCatalog(text, lang string) ([]entry, error) {
	var out []entry
	var field *string
	for n, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "#"):
		case strings.TrimSpace(line) == "":
			field = nil
		case strings.HasPrefix(line, "  |"):
			if field == nil {
				return nil, fmt.Errorf("line %d: continuation without a text before it", n+1)
			}
			*field += "\n" + line[3:]
		case strings.HasPrefix(line, "en:"):
			out = append(out, entry{en: strings.TrimPrefix(strings.TrimPrefix(line, "en:"), " ")})
			field = &out[len(out)-1].en
		case strings.HasPrefix(line, lang+":"):
			if len(out) == 0 || field == nil {
				return nil, fmt.Errorf("line %d: %s: without en: before it", n+1, lang)
			}
			out[len(out)-1].tr = strings.TrimPrefix(strings.TrimPrefix(line, lang+":"), " ")
			field = &out[len(out)-1].tr
		default:
			return nil, fmt.Errorf("line %d: expected en:, %s:, '  |', '#' or a blank line", n+1, lang)
		}
	}
	return out, nil
}

// confTemplate is nacomline.conf in a language (the English one if it has none).
func confTemplate(lang string) string {
	b, err := langFS.ReadFile("lang/nacomline.conf." + lang)
	if err != nil {
		return defaultConf
	}
	_, rest, _ := strings.Cut(string(b), "\n")
	return rest
}

// templateSource is the first line a translated template must have.
func templateSource(english string) string {
	s := sha256.Sum256([]byte(english))
	return "# source " + hex.EncodeToString(s[:8])
}
