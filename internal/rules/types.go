package rules

import "strings"

const (
	PolicyProxy  = "PROXY"
	PolicyDirect = "DIRECT"
	PolicyReject = "REJECT"

	FormatUnknown                = "UNKNOWN"
	FormatShadowrocketWithPolicy = "SHADOWROCKET_WITH_POLICY"
	FormatRuleListWithoutPolicy  = "RULE_LIST_WITHOUT_POLICY"
	FormatClashClassical         = "CLASH_CLASSICAL"
	FormatClashPayloadDomain     = "CLASH_PAYLOAD_DOMAIN"
	FormatClashPayloadCIDR       = "CLASH_PAYLOAD_CIDR"
	FormatPlainDomainList        = "PLAIN_DOMAIN_LIST"
	FormatMixed                  = "MIXED"
)

// RuleTypes lists the rule kinds understood by both Clash and Shadowrocket.
var RuleTypes = []string{
	"DOMAIN",
	"DOMAIN-SUFFIX",
	"DOMAIN-KEYWORD",
	"IP-CIDR",
	"IP-CIDR6",
	"GEOIP",
	"USER-AGENT",
	"PROCESS-NAME",
}

// Policies lists the built-in routing targets.
var Policies = []string{PolicyProxy, PolicyDirect, PolicyReject}

// SyncIntervals maps a stored interval key to its period in hours.
var SyncIntervals = map[string]int{
	"ONE_HOUR":     1,
	"SIX_HOURS":    6,
	"TWELVE_HOURS": 12,
	"DAILY":        24,
	"WEEKLY":       168,
}

// ParsedRule is one parsed rule line; an empty Policy means the line omitted one.
type ParsedRule struct {
	Type        string `json:"type"`
	Value       string `json:"value"`
	Policy      string `json:"policy,omitempty"`
	Raw         string `json:"raw,omitempty"`
	SourceOrder int    `json:"sourceOrder"`
}

// OutputRule is a normalized rule ready to be emitted into a subscription.
type OutputRule struct {
	Type   string `json:"type"`
	Value  string `json:"value"`
	Policy string `json:"policy"`
}

// ParseResult holds the parsed rules and format classification for a source body.
type ParseResult struct {
	Format       string
	HasPolicy    bool
	Rules        []ParsedRule
	InvalidLines []string
}

// AnalyzeResult reports the pre-import inspection of an external rule source.
type AnalyzeResult struct {
	Format             string       `json:"format" example:"RULE_LIST_WITHOUT_POLICY"`
	HasPolicy          bool         `json:"hasPolicy" example:"false"`
	MissingPolicyCount int          `json:"missingPolicyCount" example:"120"`
	ValidCount         int          `json:"validCount" example:"120"`
	InvalidCount       int          `json:"invalidCount" example:"0"`
	Preview            []ParsedRule `json:"preview"`
	InvalidPreview     []string     `json:"invalidPreview"`
}

// RuleSummary aggregates manual and external rule counts and policy distribution.
type RuleSummary struct {
	ManualCount   int            `json:"manualCount" example:"5"`
	ExternalCount int            `json:"externalCount" example:"120"`
	TotalCount    int            `json:"totalCount" example:"125"`
	PolicyCounts  map[string]int `json:"policyCounts"`
}

// NormalizeRuleType canonicalizes underscore variants (DOMAIN_SUFFIX) to hyphen form.
func NormalizeRuleType(value string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), "_", "-"))
}

func IsPolicy(value string) bool {
	for _, p := range Policies {
		if p == value {
			return true
		}
	}
	return false
}

func IsRuleType(value string) bool {
	canon := NormalizeRuleType(value)
	for _, t := range RuleTypes {
		if t == canon {
			return true
		}
	}
	return false
}

func IsSyncInterval(value string) bool {
	_, ok := SyncIntervals[value]
	return ok
}
