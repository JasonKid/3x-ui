package service

import (
	"strings"
	"testing"
)

// A subscription template that is not a YAML mapping used to be dropped by the
// generator without any feedback; saving it must now fail with a usable reason.
func TestUpdateAllSettingRejectsInvalidClashTemplate(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}

	cases := []struct {
		name     string
		template string
		wantErr  string
	}{
		{name: "bare scalar", template: "fjdklsfls", wantErr: "must be a YAML mapping"},
		{name: "redeclares proxies", template: "proxies: []\n", wantErr: "must not define"},
		{name: "redeclares rules", template: "dns:\n  enable: true\nrules:\n  - MATCH,DIRECT\n", wantErr: "must not define"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings, err := s.GetAllSetting()
			if err != nil {
				t.Fatal(err)
			}
			settings.SubClashTemplate = tc.template
			err = s.UpdateAllSetting(settings, SecretClears{})
			if err == nil {
				t.Fatalf("accepted an unusable Clash template %q", tc.template)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want it to mention %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestUpdateAllSettingAcceptsUsableClashTemplate(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}

	cases := []struct {
		name     string
		template string
	}{
		{name: "empty means built-in", template: ""},
		{name: "plain yaml mapping", template: "dns:\n  enable: true\nallow-lan: false\nmode: rule\n"},
		{name: "placeholder template", template: "proxies:\n{{PROXIES}}\nrules:\n{{RULES}}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings, err := s.GetAllSetting()
			if err != nil {
				t.Fatal(err)
			}
			settings.SubClashTemplate = tc.template
			if err := s.UpdateAllSetting(settings, SecretClears{}); err != nil {
				t.Fatalf("rejected a usable Clash template: %v", err)
			}
		})
	}
}

func TestUpdateAllSettingValidatesShadowrocketTemplate(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}

	rejected := []struct {
		name     string
		template string
		wantErr  string
	}{
		{name: "bare scalar", template: "fjdklsfls", wantErr: "expected `key = value`"},
		{name: "unclosed section", template: "[General\nfoo = bar\n", wantErr: "unclosed section header"},
		{name: "line before any section", template: "bypass-system = true\n", wantErr: "before any [Section]"},
	}
	for _, tc := range rejected {
		t.Run("reject "+tc.name, func(t *testing.T) {
			settings, err := s.GetAllSetting()
			if err != nil {
				t.Fatal(err)
			}
			settings.SubShadowrocketTemplate = tc.template
			err = s.UpdateAllSetting(settings, SecretClears{})
			if err == nil {
				t.Fatalf("accepted an unusable Shadowrocket template %q", tc.template)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want it to mention %q", err.Error(), tc.wantErr)
			}
		})
	}

	accepted := []struct {
		name     string
		template string
	}{
		{name: "empty means built-in", template: ""},
		{name: "general section", template: "[General]\nbypass-system = true\nipv6 = false\n"},
		{name: "with comments", template: "# comment\n[General]\n;; another\nipv6 = false\n"},
		{name: "placeholder template", template: "[General]\n{{RULES}}\n"},
	}
	for _, tc := range accepted {
		t.Run("accept "+tc.name, func(t *testing.T) {
			settings, err := s.GetAllSetting()
			if err != nil {
				t.Fatal(err)
			}
			settings.SubShadowrocketTemplate = tc.template
			if err := s.UpdateAllSetting(settings, SecretClears{}); err != nil {
				t.Fatalf("rejected a usable Shadowrocket template: %v", err)
			}
		})
	}
}
