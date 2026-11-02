package rules

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

const (
	maxSourceBytes = 10 << 20
	fetchTimeout   = 20 * time.Second
	fetchUserAgent = "clash-verge/v2.5.8"
)

// LoadOutputRules returns manual rules followed by enabled source rules,
// de-duplicated by type+value+policy.
func LoadOutputRules() ([]OutputRule, error) {
	db := database.GetDB()
	if db == nil {
		return nil, nil
	}
	var manual []model.Rule
	if err := db.Where("enabled = ?", true).Order("sort_order asc, id asc").Find(&manual).Error; err != nil {
		return nil, err
	}
	var entries []model.RuleSourceEntry
	if err := db.Table("rule_source_entries AS e").
		Joins("JOIN rule_sources AS s ON s.id = e.source_id").
		Where("e.enabled = ? AND s.enabled = ?", true, true).
		Order("s.sort_order asc, s.id asc, e.source_order asc, e.id asc").
		Find(&entries).Error; err != nil {
		return nil, err
	}

	out := make([]OutputRule, 0, len(manual)+len(entries))
	seen := make(map[string]struct{}, len(manual)+len(entries))
	add := func(rule OutputRule) {
		rule.Type = NormalizeRuleType(rule.Type)
		key := rule.Type + "|" + rule.Value + "|" + rule.Policy
		if _, dup := seen[key]; dup {
			return
		}
		seen[key] = struct{}{}
		out = append(out, rule)
	}
	for _, rule := range manual {
		add(OutputRule{Type: rule.Type, Value: rule.Value, Policy: rule.Policy})
	}
	for _, entry := range entries {
		add(OutputRule{Type: entry.Type, Value: entry.Value, Policy: entry.Policy})
	}
	return out, nil
}

// GetRuleSummary computes manual count, external count and per-policy totals.
func GetRuleSummary() (RuleSummary, error) {
	db := database.GetDB()
	summary := RuleSummary{
		PolicyCounts: map[string]int{
			PolicyProxy:  0,
			PolicyDirect: 0,
			PolicyReject: 0,
		},
	}
	var manual []model.Rule
	if err := db.Find(&manual).Error; err != nil {
		return summary, err
	}
	summary.ManualCount = len(manual)
	for _, r := range manual {
		if _, ok := summary.PolicyCounts[r.Policy]; ok {
			summary.PolicyCounts[r.Policy]++
		}
	}

	type policyRow struct {
		Policy string
		Count  int
	}
	var rows []policyRow
	if err := db.Model(&model.RuleSourceEntry{}).
		Select("policy, COUNT(*) as count").
		Group("policy").
		Scan(&rows).Error; err != nil {
		return summary, err
	}
	for _, row := range rows {
		summary.ExternalCount += row.Count
		if _, ok := summary.PolicyCounts[row.Policy]; ok {
			summary.PolicyCounts[row.Policy] += row.Count
		}
	}
	summary.TotalCount = summary.ManualCount + summary.ExternalCount
	return summary, nil
}

// FetchSource downloads a remote rule list using the Clash Verge User-Agent.
func FetchSource(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		return "", errors.New("only HTTP and HTTPS URLs are supported")
	}
	client := &http.Client{Timeout: fetchTimeout}
	req, err := http.NewRequest(http.MethodGet, trimmed, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", fetchUserAgent)
	req.Header.Set("Accept", "text/plain, application/yaml, text/yaml, application/x-yaml, */*")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSourceBytes+1))
	if err != nil {
		return "", err
	}
	if len(body) > maxSourceBytes {
		return "", errors.New("rule source is too large")
	}
	if strings.TrimSpace(string(body)) == "" {
		return "", errors.New("source returned an empty body")
	}
	return string(body), nil
}

// AnalyzeSourceURL fetches and parses a remote rule source before saving it.
func AnalyzeSourceURL(rawURL string) (*AnalyzeResult, error) {
	content, err := FetchSource(rawURL)
	if err != nil {
		return nil, err
	}
	parsed := ParseRuleSource(content)
	if len(parsed.Rules) == 0 {
		return nil, errors.New("no valid rules found")
	}
	missing := 0
	for _, r := range parsed.Rules {
		if r.Policy == "" {
			missing++
		}
	}
	preview := parsed.Rules
	if len(preview) > 10 {
		preview = preview[:10]
	}
	invalidPreview := parsed.InvalidLines
	if len(invalidPreview) > 10 {
		invalidPreview = invalidPreview[:10]
	}
	return &AnalyzeResult{
		Format:             parsed.Format,
		HasPolicy:          parsed.HasPolicy,
		MissingPolicyCount: missing,
		ValidCount:         len(parsed.Rules),
		InvalidCount:       len(parsed.InvalidLines),
		Preview:            preview,
		InvalidPreview:     invalidPreview,
	}, nil
}

// SyncSource refreshes one source: fetch, parse, deduplicate and replace its entries.
func SyncSource(source *model.RuleSource) error {
	content, err := FetchSource(source.Url)
	if err != nil {
		markSourceFailure(source, err)
		return err
	}
	result := ParseRuleSource(content)
	if len(result.Rules) == 0 {
		err := errors.New("no valid rules found")
		markSourceFailure(source, err)
		return err
	}
	policy := source.DefaultPolicy
	if !IsPolicy(policy) {
		policy = PolicyProxy
	}
	normalized := ApplyDefaultPolicy(result.Rules, policy)

	seen := make(map[string]struct{}, len(normalized))
	unique := make([]OutputRule, 0, len(normalized))
	for _, rule := range normalized {
		key := rule.Type + "|" + rule.Value + "|" + rule.Policy
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, rule)
	}

	db := database.GetDB()
	now := time.Now().UnixMilli()
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("source_id = ?", source.Id).Delete(&model.RuleSourceEntry{}).Error; err != nil {
			return err
		}
		entries := make([]model.RuleSourceEntry, 0, len(unique))
		for i, rule := range unique {
			entries = append(entries, model.RuleSourceEntry{
				SourceId:    source.Id,
				Type:        rule.Type,
				Value:       rule.Value,
				Policy:      rule.Policy,
				Enabled:     true,
				SourceOrder: i,
			})
		}
		if len(entries) > 0 {
			if err := tx.CreateInBatches(entries, 200).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.RuleSource{}).Where("id = ?", source.Id).Updates(map[string]any{
			"last_synced_at":  now,
			"last_status":     "SUCCESS",
			"last_error":      "",
			"rule_count":      len(entries),
			"detected_format": result.Format,
		}).Error
	})
	if err != nil {
		markSourceFailure(source, err)
		return err
	}
	source.LastSyncedAt = now
	source.LastStatus = "SUCCESS"
	source.LastError = ""
	source.RuleCount = len(unique)
	source.DetectedFormat = result.Format
	return nil
}

func markSourceFailure(source *model.RuleSource, cause error) {
	message := cause.Error()
	if len(message) > 500 {
		message = message[:500]
	}
	source.LastStatus = "FAILED"
	source.LastError = message
	database.GetDB().Model(&model.RuleSource{}).Where("id = ?", source.Id).Updates(map[string]any{
		"last_status": "FAILED",
		"last_error":  message,
	})
}
