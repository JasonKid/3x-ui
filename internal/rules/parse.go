package rules

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
)

var domainPattern = regexp.MustCompile(`^[a-zA-Z0-9_*.-]+\.[a-zA-Z]{2,}$`)

func stripComment(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
		return ""
	}
	return trimmed
}

func cleanPayloadItem(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) >= 2 {
		if (trimmed[0] == '\'' && trimmed[len(trimmed)-1] == '\'') ||
			(trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"') {
			trimmed = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		}
	}
	return trimmed
}

func isCIDR(value string) bool {
	address, prefix, ok := strings.Cut(value, "/")
	if !ok || strings.Contains(prefix, "/") {
		return false
	}
	ip := net.ParseIP(address)
	if ip == nil || ip.To4() == nil {
		return false
	}
	n, err := strconv.Atoi(prefix)
	return err == nil && n >= 0 && n <= 32
}

func isCIDR6(value string) bool {
	address, prefix, ok := strings.Cut(value, "/")
	if !ok || strings.Contains(prefix, "/") {
		return false
	}
	ip := net.ParseIP(address)
	if ip == nil || ip.To4() != nil {
		return false
	}
	n, err := strconv.Atoi(prefix)
	return err == nil && n >= 0 && n <= 128
}

func normalizeDomain(value string) string {
	for _, prefix := range []string{"+.", "*.", "."} {
		if strings.HasPrefix(value, prefix) {
			return strings.TrimPrefix(value, prefix)
		}
	}
	return value
}

func looksLikeDomain(value string) bool {
	return domainPattern.MatchString(normalizeDomain(value))
}

func parseRuleLine(line string, index int) (ParsedRule, bool) {
	cleaned := cleanPayloadItem(stripComment(line))
	if cleaned == "" {
		return ParsedRule{}, false
	}

	parts := strings.Split(cleaned, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if len(parts) >= 2 && IsRuleType(parts[0]) {
		if parts[1] == "" {
			return ParsedRule{}, false
		}
		rule := ParsedRule{
			Type:        NormalizeRuleType(parts[0]),
			Value:       parts[1],
			Raw:         cleaned,
			SourceOrder: index,
		}
		if len(parts) >= 3 && parts[2] != "" {
			if strings.EqualFold(parts[2], "no-resolve") {
				return rule, true
			}
			upperPolicy := strings.ToUpper(parts[2])
			if !IsPolicy(upperPolicy) {
				return ParsedRule{}, false
			}
			rule.Policy = upperPolicy
		}
		return rule, true
	}

	if isCIDR(cleaned) {
		return ParsedRule{Type: "IP-CIDR", Value: cleaned, Raw: cleaned, SourceOrder: index}, true
	}
	if isCIDR6(cleaned) {
		return ParsedRule{Type: "IP-CIDR6", Value: cleaned, Raw: cleaned, SourceOrder: index}, true
	}
	if looksLikeDomain(cleaned) {
		return ParsedRule{Type: "DOMAIN-SUFFIX", Value: normalizeDomain(cleaned), Raw: cleaned, SourceOrder: index}, true
	}
	return ParsedRule{}, false
}

func toStrings(items []any) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, fmt.Sprint(item))
	}
	return out
}

func parseYAMLPayload(content string) ([]string, bool) {
	var doc struct {
		Payload []any `yaml:"payload"`
	}
	if err := yaml.Unmarshal([]byte(content), &doc); err == nil && len(doc.Payload) > 0 {
		return toStrings(doc.Payload), true
	}
	var list []any
	if err := yaml.Unmarshal([]byte(content), &list); err == nil && len(list) > 0 {
		return toStrings(list), true
	}
	return nil, false
}

func detectFormat(parsed []ParsedRule, fromYAML bool) string {
	withPolicy := 0
	for _, rule := range parsed {
		if rule.Policy != "" {
			withPolicy++
		}
	}
	if withPolicy > 0 && withPolicy < len(parsed) {
		return FormatMixed
	}
	types := make(map[string]struct{}, len(parsed))
	for _, rule := range parsed {
		types[rule.Type] = struct{}{}
	}
	if fromYAML {
		onlyCIDR, onlyDomain := true, true
		for t := range types {
			if t != "IP-CIDR" && t != "IP-CIDR6" {
				onlyCIDR = false
			}
			if !strings.HasPrefix(t, "DOMAIN") {
				onlyDomain = false
			}
		}
		if onlyCIDR {
			return FormatClashPayloadCIDR
		}
		if onlyDomain {
			return FormatClashPayloadDomain
		}
		return FormatClashClassical
	}
	if withPolicy > 0 {
		return FormatShadowrocketWithPolicy
	}
	if len(types) == 1 {
		if _, ok := types["DOMAIN-SUFFIX"]; ok {
			return FormatPlainDomainList
		}
	}
	return FormatRuleListWithoutPolicy
}

// ParseRuleSource accepts Shadowrocket lists, Clash payload/classical lists,
// plain domain lists and mixed forms, returning every rule it could read.
func ParseRuleSource(content string) ParseResult {
	var parsed []ParsedRule
	invalid := make([]string, 0)

	if payload, ok := parseYAMLPayload(content); ok {
		for i, item := range payload {
			cleaned := cleanPayloadItem(item)
			if cleaned == "" {
				continue
			}
			if rule, ok := parseRuleLine(cleaned, i); ok {
				parsed = append(parsed, rule)
			} else {
				invalid = append(invalid, cleaned)
			}
		}
		format := FormatUnknown
		if len(parsed) > 0 {
			format = detectFormat(parsed, true)
		}
		return ParseResult{
			Format:       format,
			HasPolicy:    hasPolicy(parsed),
			Rules:        parsed,
			InvalidLines: invalid,
		}
	}

	for i, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		cleaned := stripComment(line)
		if cleaned == "" || strings.HasPrefix(cleaned, "[") || strings.HasPrefix(cleaned, "#!") {
			continue
		}
		if rule, ok := parseRuleLine(cleaned, i); ok {
			parsed = append(parsed, rule)
		} else {
			invalid = append(invalid, cleaned)
		}
	}
	format := FormatUnknown
	if len(parsed) > 0 {
		format = detectFormat(parsed, false)
	}
	return ParseResult{
		Format:       format,
		HasPolicy:    hasPolicy(parsed),
		Rules:        parsed,
		InvalidLines: invalid,
	}
}

func hasPolicy(parsed []ParsedRule) bool {
	for _, rule := range parsed {
		if rule.Policy != "" {
			return true
		}
	}
	return false
}

// ApplyDefaultPolicy fills in the policy for rules whose source had none.
func ApplyDefaultPolicy(parsed []ParsedRule, defaultPolicy string) []OutputRule {
	out := make([]OutputRule, 0, len(parsed))
	for _, rule := range parsed {
		policy := rule.Policy
		if policy == "" {
			policy = defaultPolicy
		}
		out = append(out, OutputRule{
			Type:   NormalizeRuleType(rule.Type),
			Value:  rule.Value,
			Policy: policy,
		})
	}
	return out
}
