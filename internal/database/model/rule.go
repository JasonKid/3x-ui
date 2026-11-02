package model

// Rule is one manually maintained routing rule shared by every subscription.
type Rule struct {
	Id        int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement" example:"1"`
	Type      string `json:"type" form:"type" gorm:"not null" example:"DOMAIN-SUFFIX"`
	Value     string `json:"value" form:"value" gorm:"not null" example:"google.com"`
	Policy    string `json:"policy" form:"policy" gorm:"not null" example:"PROXY"`
	Note      string `json:"note" form:"note" example:""`
	Enabled   bool   `json:"enabled" form:"enabled" gorm:"default:true" example:"true"`
	SortOrder int    `json:"sortOrder" form:"sortOrder" gorm:"default:0;index:idx_rules_sort" example:"1"`
	CreatedAt int64  `json:"createdAt" gorm:"autoCreateTime:milli" example:"1735689600000"`
	UpdatedAt int64  `json:"updatedAt" gorm:"autoUpdateTime:milli" example:"1735689600000"`
}

// RuleSource is a remote rule list the panel refreshes on a schedule.
type RuleSource struct {
	Id             int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement" example:"1"`
	Name           string `json:"name" form:"name" gorm:"not null" example:"Apple"`
	Url            string `json:"url" form:"url" gorm:"not null" example:"https://example.com/apple.list"`
	DefaultPolicy  string `json:"defaultPolicy" form:"defaultPolicy" gorm:"default:PROXY" example:"DIRECT"`
	DetectedFormat string `json:"detectedFormat" form:"detectedFormat" example:"RULE_LIST_WITHOUT_POLICY"`
	Interval       string `json:"interval" form:"interval" gorm:"default:DAILY" example:"DAILY"`
	Enabled        bool   `json:"enabled" form:"enabled" gorm:"default:true" example:"true"`
	SortOrder      int    `json:"sortOrder" form:"sortOrder" gorm:"default:0;index:idx_rule_sources_sort" example:"1"`
	LastSyncedAt   int64  `json:"lastSyncedAt" form:"lastSyncedAt" example:"1735689600000"`
	LastStatus     string `json:"lastStatus" form:"lastStatus" gorm:"default:NEVER" example:"SUCCESS"`
	LastError      string `json:"lastError" form:"lastError" gorm:"type:text" example:""`
	RuleCount      int    `json:"ruleCount" form:"ruleCount" gorm:"default:0" example:"120"`
	CreatedAt      int64  `json:"createdAt" gorm:"autoCreateTime:milli" example:"1735689600000"`
	UpdatedAt      int64  `json:"updatedAt" gorm:"autoUpdateTime:milli" example:"1735689600000"`
}

// RuleSourceEntry is one rule parsed out of a remote source.
type RuleSourceEntry struct {
	Id          int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	SourceId    int    `json:"sourceId" form:"sourceId" gorm:"not null;uniqueIndex:idx_rule_entry_identity,priority:1;index:idx_rule_entries_source"`
	Type        string `json:"type" form:"type" gorm:"not null;uniqueIndex:idx_rule_entry_identity,priority:2"`
	Value       string `json:"value" form:"value" gorm:"not null;uniqueIndex:idx_rule_entry_identity,priority:3"`
	Policy      string `json:"policy" form:"policy" gorm:"not null;uniqueIndex:idx_rule_entry_identity,priority:4"`
	Enabled     bool   `json:"enabled" form:"enabled" gorm:"default:true"`
	SourceOrder int    `json:"sourceOrder" form:"sourceOrder" gorm:"default:0"`
	Raw         string `json:"raw" form:"raw" gorm:"type:text"`
	CreatedAt   int64  `json:"createdAt" gorm:"autoCreateTime:milli"`
	UpdatedAt   int64  `json:"updatedAt" gorm:"autoUpdateTime:milli"`
}
