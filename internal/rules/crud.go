package rules

import (
	"errors"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

// ErrInvalidRule reports a rule or source that fails validation.
var ErrInvalidRule = errors.New("invalid rule")

// Normalize validates a manual rule and canonicalizes its fields.
func Normalize(rule *model.Rule) error {
	rule.Type = NormalizeRuleType(rule.Type)
	rule.Value = strings.TrimSpace(rule.Value)
	rule.Policy = strings.ToUpper(strings.TrimSpace(rule.Policy))
	rule.Note = strings.TrimSpace(rule.Note)
	if !IsRuleType(rule.Type) || rule.Value == "" || len(rule.Value) > 512 || strings.ContainsAny(rule.Value, "\r\n,") || !IsPolicy(rule.Policy) {
		return ErrInvalidRule
	}
	return nil
}

func ListRules() ([]model.Rule, error) {
	var rows []model.Rule
	err := database.GetDB().Order("sort_order asc, id asc").Find(&rows).Error
	return rows, err
}

// CreateRule inserts a manual rule at the top of the evaluation order (sort_order = 1).
func CreateRule(rule *model.Rule) error {
	if err := Normalize(rule); err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	rule.CreatedAt = now
	rule.UpdatedAt = now
	rule.SortOrder = 1
	db := database.GetDB()
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Rule{}).Where("1 = 1").
			Update("sort_order", gorm.Expr("sort_order + 1")).Error; err != nil {
			return err
		}
		return tx.Create(rule).Error
	})
}

func UpdateRule(rule *model.Rule) error {
	if err := Normalize(rule); err != nil {
		return err
	}
	rule.UpdatedAt = time.Now().UnixMilli()
	return database.GetDB().Model(&model.Rule{}).Where("id = ?", rule.Id).Updates(map[string]any{
		"type":       rule.Type,
		"value":      rule.Value,
		"policy":     rule.Policy,
		"note":       rule.Note,
		"enabled":    rule.Enabled,
		"updated_at": rule.UpdatedAt,
	}).Error
}

func DeleteRule(id int) error {
	return database.GetDB().Where("id = ?", id).Delete(&model.Rule{}).Error
}

// ReorderRules writes the given id order into sort_order.
func ReorderRules(ids []int) error {
	db := database.GetDB()
	return db.Transaction(func(tx *gorm.DB) error {
		for index, id := range ids {
			if err := tx.Model(&model.Rule{}).Where("id = ?", id).
				Update("sort_order", index+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func ListSources() ([]model.RuleSource, error) {
	var rows []model.RuleSource
	err := database.GetDB().Order("sort_order asc, id asc").Find(&rows).Error
	return rows, err
}

// NormalizeSource validates a source and fills policy/interval defaults.
func NormalizeSource(source *model.RuleSource) error {
	source.Name = strings.TrimSpace(source.Name)
	source.Url = strings.TrimSpace(source.Url)
	source.DefaultPolicy = strings.ToUpper(strings.TrimSpace(source.DefaultPolicy))
	source.Interval = strings.ToUpper(strings.TrimSpace(source.Interval))
	if source.Name == "" || len(source.Name) > 120 || source.Url == "" {
		return ErrInvalidRule
	}
	if !strings.HasPrefix(source.Url, "http://") && !strings.HasPrefix(source.Url, "https://") {
		return ErrInvalidRule
	}
	if source.DefaultPolicy == "" {
		source.DefaultPolicy = PolicyProxy
	}
	if !IsPolicy(source.DefaultPolicy) {
		return ErrInvalidRule
	}
	if source.Interval == "" {
		source.Interval = "DAILY"
	}
	if !IsSyncInterval(source.Interval) {
		return ErrInvalidRule
	}
	if source.LastStatus == "" {
		source.LastStatus = "NEVER"
	}
	return nil
}

// CreateSource inserts a source at the top of the list and immediately syncs it.
func CreateSource(source *model.RuleSource) error {
	if err := NormalizeSource(source); err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	source.CreatedAt = now
	source.UpdatedAt = now
	source.SortOrder = 1
	db := database.GetDB()
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.RuleSource{}).Where("1 = 1").
			Update("sort_order", gorm.Expr("sort_order + 1")).Error; err != nil {
			return err
		}
		return tx.Create(source).Error
	})
	if err != nil {
		return err
	}
	_ = SyncSource(source)
	return nil
}

// UpdateSource updates a source and re-syncs if its URL or default policy changed.
func UpdateSource(source *model.RuleSource) error {
	existing, err := GetSource(source.Id)
	if err != nil {
		return err
	}
	if err := NormalizeSource(source); err != nil {
		return err
	}
	needSync := existing.Url != source.Url || existing.DefaultPolicy != source.DefaultPolicy
	source.UpdatedAt = time.Now().UnixMilli()
	if err := database.GetDB().Model(&model.RuleSource{}).Where("id = ?", source.Id).Updates(map[string]any{
		"name":           source.Name,
		"url":            source.Url,
		"default_policy": source.DefaultPolicy,
		"interval":       source.Interval,
		"enabled":        source.Enabled,
		"updated_at":     source.UpdatedAt,
	}).Error; err != nil {
		return err
	}
	if needSync {
		_ = SyncSource(source)
	}
	if refreshed, rErr := GetSource(source.Id); rErr == nil {
		*source = *refreshed
	}
	return nil
}

func DeleteSource(id int) error {
	db := database.GetDB()
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("source_id = ?", id).Delete(&model.RuleSourceEntry{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&model.RuleSource{}).Error
	})
}

// ReorderSources writes the given id order into sort_order for external sources.
func ReorderSources(ids []int) error {
	db := database.GetDB()
	return db.Transaction(func(tx *gorm.DB) error {
		for index, id := range ids {
			if err := tx.Model(&model.RuleSource{}).Where("id = ?", id).
				Update("sort_order", index+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func GetSource(id int) (*model.RuleSource, error) {
	var source model.RuleSource
	if err := database.GetDB().First(&source, id).Error; err != nil {
		return nil, err
	}
	return &source, nil
}

// ListSourceEntries pages one source's parsed rules.
func ListSourceEntries(sourceId, offset, limit int) ([]model.RuleSourceEntry, int64, error) {
	db := database.GetDB()
	var total int64
	if err := db.Model(&model.RuleSourceEntry{}).Where("source_id = ?", sourceId).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.RuleSourceEntry
	err := db.Where("source_id = ?", sourceId).
		Order("source_order asc, id asc").
		Offset(offset).Limit(limit).Find(&rows).Error
	return rows, total, err
}

// SyncDue refreshes every enabled source whose interval has elapsed.
func SyncDue() {
	sources, err := ListSources()
	if err != nil {
		return
	}
	now := time.Now().UnixMilli()
	for i := range sources {
		source := &sources[i]
		if !source.Enabled {
			continue
		}
		hours, ok := SyncIntervals[source.Interval]
		if !ok {
			continue
		}
		if source.LastSyncedAt > 0 && now-source.LastSyncedAt < int64(hours)*int64(time.Hour/time.Millisecond) {
			continue
		}
		_ = SyncSource(source)
	}
}
