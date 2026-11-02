package rules

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestParseRuleSourceFormats(t *testing.T) {
	t.Run("plain domain list", func(t *testing.T) {
		res := ParseRuleSource("# comment\n+.google.com\n*.github.com\nexample.org\n")
		if res.Format != FormatPlainDomainList {
			t.Fatalf("Format = %q, want %q", res.Format, FormatPlainDomainList)
		}
		if res.HasPolicy || len(res.Rules) != 3 {
			t.Fatalf("HasPolicy=%v len=%d", res.HasPolicy, len(res.Rules))
		}
		if res.Rules[0].Type != "DOMAIN-SUFFIX" || res.Rules[0].Value != "google.com" {
			t.Fatalf("rule[0] = %#v", res.Rules[0])
		}
	})

	t.Run("clash payload yaml", func(t *testing.T) {
		res := ParseRuleSource("payload:\n  - 'DOMAIN-SUFFIX,openai.com'\n  - 'IP-CIDR,1.1.1.0/24,no-resolve'\n")
		if res.Format != FormatClashClassical {
			t.Fatalf("Format = %q, want %q", res.Format, FormatClashClassical)
		}
		if len(res.Rules) != 2 || res.Rules[1].Type != "IP-CIDR" {
			t.Fatalf("rules = %#v", res.Rules)
		}
	})

	t.Run("shadowrocket with policy", func(t *testing.T) {
		res := ParseRuleSource("DOMAIN,ads.example.com,REJECT\nDOMAIN-SUFFIX,cn,DIRECT\n")
		if res.Format != FormatShadowrocketWithPolicy || !res.HasPolicy {
			t.Fatalf("Format=%q HasPolicy=%v", res.Format, res.HasPolicy)
		}
	})
}

func TestRulesStoreAndTemplateRender(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))

	r1 := &model.Rule{Type: "DOMAIN-SUFFIX", Value: "first.example.com", Policy: "PROXY", Enabled: true}
	r2 := &model.Rule{Type: "DOMAIN", Value: "second.example.com", Policy: "REJECT", Enabled: true}
	if err := CreateRule(r1); err != nil {
		t.Fatalf("CreateRule r1: %v", err)
	}
	if err := CreateRule(r2); err != nil {
		t.Fatalf("CreateRule r2: %v", err)
	}

	out, err := LoadOutputRules()
	if err != nil {
		t.Fatalf("LoadOutputRules: %v", err)
	}
	if len(out) != 2 || out[0].Value != "second.example.com" || out[1].Value != "first.example.com" {
		t.Fatalf("unexpected rule order: %#v", out)
	}

	if err := ReorderRules([]int{r1.Id, r2.Id}); err != nil {
		t.Fatalf("ReorderRules: %v", err)
	}
	out, err = LoadOutputRules()
	if err != nil {
		t.Fatalf("LoadOutputRules after reorder: %v", err)
	}
	if out[0].Value != "first.example.com" || out[1].Value != "second.example.com" {
		t.Fatalf("order after ReorderRules = %#v", out)
	}

	summary, err := GetRuleSummary()
	if err != nil {
		t.Fatalf("GetRuleSummary: %v", err)
	}
	if summary.ManualCount != 2 || summary.PolicyCounts["PROXY"] != 1 || summary.PolicyCounts["REJECT"] != 1 {
		t.Fatalf("summary = %#v", summary)
	}

	cfg := map[string]any{
		"proxies": []map[string]any{{"name": "node-1", "type": "vless"}},
	}
	ApplyClashTemplateAndRules(cfg, DefaultClashTemplate, out)
	rulesSlice, ok := cfg["rules"].([]string)
	if !ok || len(rulesSlice) != 3 || rulesSlice[0] != "DOMAIN-SUFFIX,first.example.com,PROXY" || rulesSlice[2] != "MATCH,DIRECT" {
		t.Fatalf("clash rules = %#v", cfg["rules"])
	}

	sr := RenderShadowrocketConfig("", []string{"node-1 = vless, 1.2.3.4, 443, password=uuid"}, []string{"node-1", "DIRECT"}, out, "https://example.com/sub/s1?flag=shadowrocket")
	if !strings.Contains(sr, "DOMAIN-SUFFIX,first.example.com,PROXY") || !strings.Contains(sr, "FINAL,DIRECT") || !strings.Contains(sr, "PROXY = select,node-1,DIRECT") {
		t.Fatalf("shadowrocket config missing expected sections:\n%s", sr)
	}
}
