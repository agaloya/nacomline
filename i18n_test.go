package main

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

var verbs = regexp.MustCompile(`%[-+# 0-9.*]*[a-zA-Z%]`)

// TestCatalogs: every lang/<code>.txt parses, and each translation keeps the English
// text's format verbs in the same order.
func TestCatalogs(t *testing.T) {
	es, _ := langFS.ReadDir("lang")
	for _, e := range es {
		code, ok := strings.CutSuffix(e.Name(), ".txt")
		if !ok || code == "strings" {
			continue
		}
		b, _ := langFS.ReadFile("lang/" + e.Name())
		cat, err := parseCatalog(string(b), code)
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		for _, c := range cat {
			if c.tr == "" {
				continue
			}
			if a, b := verbs.FindAllString(c.en, -1), verbs.FindAllString(c.tr, -1); !slices.Equal(a, b) {
				t.Errorf("%s: %q has %v, its translation %v", e.Name(), c.en, a, b)
			}
		}
	}
}

var general = regexp.MustCompile(`(?m)^\[general\]\n(#[^\n]*\n)+language =\n`)

// TestConfTranslations: every lang/nacomline.conf.<code> translates the current template
// and has its sections, settings and default values, and the same language block.
func TestConfTranslations(t *testing.T) {
	en, err := parseINI(defaultConf)
	if err != nil {
		t.Fatal(err)
	}
	es, _ := langFS.ReadDir("lang")
	for _, e := range es {
		code, ok := strings.CutPrefix(e.Name(), "nacomline.conf.")
		if !ok {
			continue
		}
		b, _ := langFS.ReadFile("lang/" + e.Name())
		first, body, _ := strings.Cut(string(b), "\n")
		if first != templateSource(defaultConf) {
			t.Errorf("%s translates an older template (%s, now %s): go run ./i18nextract -check %s", e.Name(), first, templateSource(defaultConf), code)
		}
		tr, err := parseINI(body)
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		for k, v := range en {
			if tr[k] != v {
				t.Errorf("%s: %s = %q, English %q", e.Name(), k, tr[k], v)
			}
		}
		if len(tr) != len(en) {
			t.Errorf("%s: %d settings, English %d", e.Name(), len(tr), len(en))
		}
		if general.FindString(body) != general.FindString(defaultConf) {
			t.Errorf("%s: the [general] language block differs from the English one (it is not translated)", e.Name())
		}
	}
}

// TestLanguageDefault: no catalog for the language, or English: texts stay English.
func TestLanguageDefault(t *testing.T) {
	if l := setLang("xx"); l != "en" || T("error:") != "error:" {
		t.Fatalf("unknown language: %s", l)
	}
	if l := setLang("pt"); l != "pt" { // Matriline's programs get it even without a catalog here
		t.Fatalf("unknown language: %s", l)
	}
	if l := setLang("en"); l != "en" {
		t.Fatalf("en: %s", l)
	}
}
