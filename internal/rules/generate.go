package rules

import (
	"strings"

	"github.com/goccy/go-yaml"
)

// DefaultClashTemplate is the built-in Clash/Mihomo subscription template.
const DefaultClashTemplate = `dns:
  enable: true
  listen: 0.0.0.0:1053
  enhanced-mode: fake-ip
  fake-ip-range: 198.18.0.1/16
  default-nameserver:
    - 223.5.5.5
    - 119.29.29.29
  proxy-server-nameserver:
    - https://doh.pub/dns-query
    - https://dns.alidns.com/dns-query
  direct-nameserver:
    - https://doh.pub/dns-query
    - https://dns.alidns.com/dns-query
  fake-ip-filter:
    - "*.lan"
    - "*.local"
    - "*.arpa"
    - localhost.ptlogin2.qq.com
    - "+.msftconnecttest.com"
    - "+.msftncsi.com"
    - "+.stun.*.*"
    - time.*.com
    - time.*.apple.com
    - "+.ntp.org.cn"
    - "+.market.xiaomi.com"
  nameserver:
    - https://doh.pub/dns-query
    - https://dns.alidns.com/dns-query
  fallback:
    - https://dns.cloudflare.com/dns-query
    - https://dns.google/dns-query
  fallback-filter:
    geoip: true
    geoip-code: CN
    ipcidr:
      - 240.0.0.0/4
      - 0.0.0.0/32
      - 127.0.0.1/32
rules:
  - MATCH,DIRECT
`

// DefaultShadowrocketTemplate is the built-in Shadowrocket subscription template.
const DefaultShadowrocketTemplate = `[General]
bypass-system = true
skip-proxy = 192.168.0.0/16,10.0.0.0/8,172.16.0.0/12,100.64.0.0/10,127.0.0.0/8,169.254.0.0/16,localhost,*.lan,*.local,*.arpa,captive.apple.com
bypass-tun = 10.0.0.0/8,100.64.0.0/10,127.0.0.0/8,169.254.0.0/16,172.16.0.0/12,192.0.0.0/24,192.0.2.0/24,192.88.99.0/24,192.168.0.0/16,198.18.0.0/15,198.51.100.0/24,203.0.113.0/24,224.0.0/4,255.255.255.255/32
dns-server = https://doh.pub/dns-query,https://dns.alidns.com/dns-query
fallback-dns-server = https://dns.cloudflare.com/dns-query,https://dns.google/dns-query
ipv6 = false
prefer-ipv6 = false
private-ip-answer = true
`

// RuleLines formats OutputRules into `TYPE,VALUE,POLICY` entries.
func RuleLines(list []OutputRule) []string {
	out := make([]string, 0, len(list))
	for _, rule := range list {
		ruleType := NormalizeRuleType(rule.Type)
		line := ruleType + "," + rule.Value + "," + rule.Policy
		switch ruleType {
		case "IP-CIDR", "IP-CIDR6", "GEOIP":
			// Resolving before an IP rule match yields a fake-ip address and breaks the match.
			line += ",no-resolve"
		}
		out = append(out, line)
	}
	return out
}

func substituteBlock(tmpl, marker string, lines []string) string {
	for {
		idx := strings.Index(tmpl, marker)
		if idx < 0 {
			return tmpl
		}
		lineStart := strings.LastIndex(tmpl[:idx], "\n") + 1
		indent := tmpl[lineStart:idx]
		if strings.TrimSpace(indent) != "" {
			return strings.Replace(tmpl, marker, strings.Join(lines, "\n"), 1)
		}
		var b strings.Builder
		for i, line := range lines {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(indent)
			b.WriteString(line)
		}
		tmpl = tmpl[:lineStart] + b.String() + tmpl[idx+len(marker):]
	}
}

// HasClashPlaceholders reports whether the template uses explicit placeholder tokens.
func HasClashPlaceholders(tmpl string) bool {
	return strings.Contains(tmpl, "{{PROXIES}}") ||
		strings.Contains(tmpl, "{{PROXY_NAMES}}") ||
		strings.Contains(tmpl, "{{RULES}}")
}

// HasShadowrocketPlaceholders reports whether the template uses placeholder tokens.
func HasShadowrocketPlaceholders(tmpl string) bool {
	return strings.Contains(tmpl, "{{PROXIES}}") ||
		strings.Contains(tmpl, "{{PROXY_GROUP}}") ||
		strings.Contains(tmpl, "{{RULES}}") ||
		strings.Contains(tmpl, "{{UPDATE_URL}}")
}

// RenderClashPlaceholders substitutes {{PROXIES}}, {{PROXY_NAMES}} and {{RULES}}.
func RenderClashPlaceholders(tmpl string, proxyLines []string, proxyNames []string, list []OutputRule) string {
	nameLines := make([]string, 0, len(proxyNames))
	for _, name := range proxyNames {
		nameLines = append(nameLines, "- "+name)
	}
	ruleEntries := make([]string, 0, len(list)+1)
	for _, line := range RuleLines(list) {
		ruleEntries = append(ruleEntries, "- "+line)
	}
	if !strings.Contains(tmpl, "MATCH,") {
		ruleEntries = append(ruleEntries, "- MATCH,DIRECT")
	}
	out := substituteBlock(tmpl, "{{PROXIES}}", proxyLines)
	out = substituteBlock(out, "{{PROXY_NAMES}}", nameLines)
	return substituteBlock(out, "{{RULES}}", ruleEntries)
}

// ApplyClashTemplateAndRules merges a YAML Clash template and generated rules into config.
func ApplyClashTemplateAndRules(config map[string]any, tmpl string, list []OutputRule) {
	trimmed := strings.TrimSpace(tmpl)
	var templateRules []string
	if trimmed != "" {
		var parsed map[string]any
		if err := yaml.Unmarshal([]byte(trimmed), &parsed); err == nil && parsed != nil {
			for k, v := range parsed {
				if k == "proxies" {
					continue
				}
				if k == "rules" {
					if items, ok := v.([]any); ok {
						for _, item := range items {
							if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
								templateRules = append(templateRules, strings.TrimSpace(s))
							}
						}
					}
					continue
				}
				config[k] = v
			}
		}
	}

	generated := RuleLines(list)
	combined := make([]string, 0, len(generated)+len(templateRules)+1)
	combined = append(combined, generated...)
	hasMatch := false
	for _, r := range templateRules {
		if strings.HasPrefix(strings.ToUpper(r), "MATCH,") {
			hasMatch = true
		}
		combined = append(combined, r)
	}
	if !hasMatch {
		if trimmed != "" || len(generated) > 0 {
			combined = append(combined, "MATCH,DIRECT")
		} else {
			combined = append(combined, "MATCH,PROXY")
		}
	}
	config["rules"] = combined
}

// RenderShadowrocketConfig builds a complete Shadowrocket configuration from a
// template (either [General] key=value lines or full INI sections/placeholders),
// generated proxy lines, proxy group members and routing rules.
func RenderShadowrocketConfig(tmpl string, proxyLines []string, proxyNames []string, list []OutputRule, updateURL string) string {
	trimmed := strings.TrimSpace(tmpl)
	if trimmed == "" {
		trimmed = strings.TrimSpace(DefaultShadowrocketTemplate)
	}

	groupMembers := strings.Join(proxyNames, ",")
	rules := RuleLines(list)

	if strings.Contains(trimmed, "{{PROXIES}}") || strings.Contains(trimmed, "{{RULES}}") || strings.Contains(trimmed, "{{PROXY_GROUP}}") {
		out := substituteBlock(trimmed, "{{PROXIES}}", proxyLines)
		out = strings.ReplaceAll(out, "{{PROXY_GROUP}}", groupMembers)
		out = strings.ReplaceAll(out, "{{UPDATE_URL}}", updateURL)
		out = substituteBlock(out, "{{RULES}}", rules)
		if !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		return out
	}

	var generalLines []string
	var extraRuleLines []string
	section := "general"
	hasUpdateURL := false

	for _, rawLine := range strings.Split(strings.ReplaceAll(trimmed, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sec := strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			section = sec
			continue
		}
		switch section {
		case "general":
			if line == "" && len(generalLines) == 0 {
				continue
			}
			if strings.HasPrefix(strings.ToLower(line), "update-url") {
				hasUpdateURL = true
			}
			generalLines = append(generalLines, rawLine)
		case "rule":
			if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, ";") {
				extraRuleLines = append(extraRuleLines, line)
			}
		}
	}

	if updateURL != "" && !hasUpdateURL {
		generalLines = append(generalLines, "update-url = "+updateURL)
	}

	hasFinal := false
	for _, r := range extraRuleLines {
		if strings.HasPrefix(strings.ToUpper(r), "FINAL,") {
			hasFinal = true
		}
	}
	if !hasFinal {
		extraRuleLines = append(extraRuleLines, "FINAL,DIRECT")
	}

	var out []string
	out = append(out, "# Shadowrocket Configuration", "", "[General]")
	out = append(out, generalLines...)
	out = append(out, "", "[Proxy]")
	out = append(out, proxyLines...)
	out = append(out, "", "[Proxy Group]")
	out = append(out, "PROXY = select,"+groupMembers)
	out = append(out, "", "[Rule]")
	out = append(out, rules...)
	out = append(out, extraRuleLines...)

	return strings.Join(out, "\n") + "\n"
}
