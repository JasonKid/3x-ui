// Command rules-migrate copies routing rules from a rules-manager SQLite
// database into the 3x-ui panel database.
package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/rules"

	_ "github.com/mattn/go-sqlite3"
)

type manualRule struct {
	Type      string
	Value     string
	Policy    string
	Note      sql.NullString
	Enabled   bool
	SortOrder int
}

type source struct {
	Id             string
	Name           string
	Url            string
	DefaultPolicy  string
	DetectedFormat sql.NullString
	Interval       string
	Enabled        bool
	SortOrder      int
	LastSyncedAt   sql.NullTime
	LastStatus     string
	LastError      sql.NullString
	RuleCount      int
	UserId         string
}

type externalRule struct {
	SourceId    string
	Type        string
	Value       string
	Policy      string
	Enabled     bool
	SourceOrder int
	Raw         sql.NullString
}

func main() {
	var (
		sourcePath  = flag.String("source", "", "path to the rules-manager SQLite database (prisma/dev.db)")
		targetPath  = flag.String("target", "", "path to the 3x-ui panel database (x-ui.db)")
		user        = flag.String("user", "", "migrate only this rules-manager username; omit to choose interactively")
		listUsers   = flag.Bool("list-users", false, "list rules-manager users with their rule counts and exit")
		dryRun      = flag.Bool("dry-run", false, "print what would be migrated without writing")
		skipSources = flag.Bool("skip-sources", false, "migrate manual rules only, skipping external rule sources")
		replace     = flag.Bool("replace", false, "delete existing manual rules and sources before importing")
	)
	flag.Parse()

	if *sourcePath == "" {
		fmt.Fprintln(os.Stderr, "usage: rules-migrate -source <rules-manager.db> [-target <x-ui.db>] [-user name] [-list-users] [-dry-run] [-replace]")
		os.Exit(2)
	}
	if _, err := os.Stat(*sourcePath); err != nil {
		fatalf("source database is not readable: %v", err)
	}

	src, err := sql.Open("sqlite3", "file:"+*sourcePath+"?mode=ro")
	if err != nil {
		fatalf("open source: %v", err)
	}
	defer src.Close()

	if *listUsers {
		printUsers(src)
		return
	}
	if *user == "" {
		selected, err := chooseUser(src)
		if err != nil {
			fatalf("%v", err)
		}
		*user = selected
	}
	if *targetPath == "" {
		fatalf("-target is required when importing (use -list-users to inspect the source)")
	}
	known, knownErr := loadSourceUsers(src)
	if knownErr != nil {
		fatalf("read users: %v", knownErr)
	}
	if !userExists(known, *user) {
		fatalf("no user %q in source database (available: %s)", *user, userNames(known))
	}
	fmt.Printf("importing rules owned by %q\n", *user)

	manual, err := loadManualRules(src, *user)
	if err != nil {
		fatalf("read manual rules: %v", err)
	}
	var sources []source
	var external []externalRule
	if !*skipSources {
		if sources, err = loadSources(src, *user); err != nil {
			fatalf("read external sources: %v", err)
		}
		if external, err = loadExternalRules(src, *user); err != nil {
			fatalf("read external rules: %v", err)
		}
	}

	fmt.Printf("found %d manual rules, %d sources, %d external rules\n", len(manual), len(sources), len(external))
	if *dryRun {
		for _, r := range manual {
			fmt.Printf("  [manual] %-14s %-30s %s enabled=%v\n", r.Type, r.Value, r.Policy, r.Enabled)
		}
		for _, s := range sources {
			fmt.Printf("  [source] %q %s (%d rules)\n", s.Name, s.Url, len(rulesOf(external, s.Id)))
		}
		fmt.Println("dry run: nothing written")
		return
	}

	if err := database.InitDB(*targetPath); err != nil {
		fatalf("open target database: %v", err)
	}
	db := database.GetDB()
	now := time.Now().UnixMilli()

	if *replace {
		fmt.Println("clearing existing manual rules and sources")
		if err := db.Where("1 = 1").Delete(&model.Rule{}).Error; err != nil {
			fatalf("clear manual rules: %v", err)
		}
		if err := db.Where("1 = 1").Delete(&model.RuleSourceEntry{}).Error; err != nil {
			fatalf("clear source entries: %v", err)
		}
		if err := db.Where("1 = 1").Delete(&model.RuleSource{}).Error; err != nil {
			fatalf("clear sources: %v", err)
		}
	}

	insertedManual, skippedManual := 0, 0
	for i, r := range manual {
		rule := &model.Rule{
			Type:      rules.NormalizeRuleType(r.Type),
			Value:     strings.TrimSpace(r.Value),
			Policy:    strings.ToUpper(strings.TrimSpace(r.Policy)),
			Note:      strings.TrimSpace(r.Note.String),
			Enabled:   r.Enabled,
			SortOrder: r.SortOrder,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if rule.SortOrder <= 0 {
			rule.SortOrder = i + 1
		}
		if err := rules.Normalize(rule); err != nil {
			skippedManual++
			fmt.Fprintf(os.Stderr, "skip manual rule %s,%s: %v\n", r.Type, r.Value, err)
			continue
		}
		res := db.Model(&model.Rule{}).Create(map[string]any{
			"type":       rule.Type,
			"value":      rule.Value,
			"policy":     rule.Policy,
			"note":       rule.Note,
			"enabled":    rule.Enabled,
			"sort_order": rule.SortOrder,
			"created_at": rule.CreatedAt,
			"updated_at": rule.UpdatedAt,
		})
		if res.Error != nil {
			skippedManual++
			fmt.Fprintf(os.Stderr, "insert manual rule %s,%s: %v\n", r.Type, r.Value, res.Error)
			continue
		}
		insertedManual++
	}

	insertedSources, skippedSources := 0, 0
	insertedEntries := 0
	for _, s := range sources {
		entryList := rulesOf(external, s.Id)
		record := &model.RuleSource{
			Name:           strings.TrimSpace(s.Name),
			Url:            strings.TrimSpace(s.Url),
			DefaultPolicy:  strings.ToUpper(strings.TrimSpace(s.DefaultPolicy)),
			DetectedFormat: s.DetectedFormat.String,
			Interval:       s.Interval,
			Enabled:        s.Enabled,
			SortOrder:      s.SortOrder,
			LastStatus:     normalizeStatus(s.LastStatus),
			LastError:      truncate(s.LastError.String, 500),
			RuleCount:      len(entryList),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if s.LastSyncedAt.Valid {
			record.LastSyncedAt = s.LastSyncedAt.Time.UnixMilli()
		}
		if err := rules.NormalizeSource(record); err != nil {
			skippedSources++
			fmt.Fprintf(os.Stderr, "skip source %q: %v\n", s.Name, err)
			continue
		}
		srcRes := db.Model(&model.RuleSource{}).Create(map[string]any{
			"name":            record.Name,
			"url":             record.Url,
			"default_policy":  record.DefaultPolicy,
			"detected_format": record.DetectedFormat,
			"interval":        record.Interval,
			"enabled":         record.Enabled,
			"sort_order":      record.SortOrder,
			"last_synced_at":  record.LastSyncedAt,
			"last_status":     record.LastStatus,
			"last_error":      record.LastError,
			"rule_count":      record.RuleCount,
			"created_at":      record.CreatedAt,
			"updated_at":      record.UpdatedAt,
		})
		if srcRes.Error != nil {
			skippedSources++
			fmt.Fprintf(os.Stderr, "insert source %q: %v\n", s.Name, srcRes.Error)
			continue
		}
		var newId int
		db.Model(&model.RuleSource{}).Select("id").Where("name = ? AND url = ?", record.Name, record.Url).
			Order("id desc").Limit(1).Scan(&newId)
		record.Id = newId
		insertedSources++

		seen := make(map[string]struct{}, len(entryList))
		for i, e := range entryList {
			entry := model.RuleSourceEntry{
				SourceId:    record.Id,
				Type:        rules.NormalizeRuleType(e.Type),
				Value:       strings.TrimSpace(e.Value),
				Policy:      strings.ToUpper(strings.TrimSpace(e.Policy)),
				Enabled:     e.Enabled,
				SourceOrder: e.SourceOrder,
				Raw:         e.Raw.String,
			}
			if entry.SourceOrder == 0 {
				entry.SourceOrder = i
			}
			if !rules.IsRuleType(entry.Type) || entry.Value == "" || !rules.IsPolicy(entry.Policy) {
				continue
			}
			key := entry.Type + "|" + entry.Value + "|" + entry.Policy
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			entryRes := db.Model(&model.RuleSourceEntry{}).Create(map[string]any{
				"source_id":    entry.SourceId,
				"type":         entry.Type,
				"value":        entry.Value,
				"policy":       entry.Policy,
				"enabled":      entry.Enabled,
				"source_order": entry.SourceOrder,
				"raw":          entry.Raw,
				"created_at":   now,
				"updated_at":   now,
			})
			if entryRes.Error != nil {
				continue
			}
			insertedEntries++
		}
	}

	fmt.Printf("migrated: %d manual rules (%d skipped), %d sources (%d skipped), %d source rules\n",
		insertedManual, skippedManual, insertedSources, skippedSources, insertedEntries)
}

// boolFromAny accepts the bool or integer forms SQLite drivers return for a
// BOOLEAN column.
func boolFromAny(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case int64:
		return typed != 0
	case int:
		return typed != 0
	case []byte:
		return string(typed) != "0" && string(typed) != "false"
	case string:
		return typed != "0" && typed != "false"
	default:
		return false
	}
}

type sourceUser struct {
	Username      string
	DisplayName   string
	ManualRules   int
	Sources       int
	ExternalRules int
}

func loadSourceUsers(db *sql.DB) ([]sourceUser, error) {
	rows, err := db.Query(`SELECT u.username, COALESCE(u.displayName, ''),
	        (SELECT COUNT(*) FROM ManualRule m WHERE m.userId = u.id),
	        (SELECT COUNT(*) FROM ExternalSource s WHERE s.userId = u.id),
	        (SELECT COUNT(*) FROM ExternalRule e WHERE e.userId = u.id)
	    FROM User u ORDER BY u.username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sourceUser
	for rows.Next() {
		var u sourceUser
		if err := rows.Scan(&u.Username, &u.DisplayName, &u.ManualRules, &u.Sources, &u.ExternalRules); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func printUsers(db *sql.DB) {
	users, err := loadSourceUsers(db)
	if err != nil {
		fatalf("read users: %v", err)
	}
	if len(users) == 0 {
		fmt.Println("no users found in source database")
		return
	}
	fmt.Printf("%-3s %-20s %-20s %8s %8s %8s\n", "#", "username", "display name", "manual", "sources", "external")
	for i, u := range users {
		fmt.Printf("%-3d %-20s %-20s %8d %8d %8d\n", i+1, u.Username, u.DisplayName, u.ManualRules, u.Sources, u.ExternalRules)
	}
}

// chooseUser asks which rules-manager account to import. A non-interactive
// stdin (CI, piped input) fails instead of silently importing every user.
func chooseUser(db *sql.DB) (string, error) {
	users, err := loadSourceUsers(db)
	if err != nil {
		return "", fmt.Errorf("read users: %w", err)
	}
	if len(users) == 0 {
		return "", fmt.Errorf("no users found in source database")
	}
	if len(users) == 1 {
		fmt.Printf("only one user in source database: %q - importing it\n", users[0].Username)
		return users[0].Username, nil
	}

	fmt.Println("rules-manager users in this database:")
	printUsers(db)
	fmt.Println()
	fmt.Printf("Choose a user to import [1-%d] (or press Enter for 1): ", len(users))

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read selection: %w", err)
	}
	if err == io.EOF && strings.TrimSpace(line) == "" {
		return "", fmt.Errorf("no selection received on stdin; pass -user <name> to choose non-interactively")
	}
	choice := strings.TrimSpace(line)
	if choice == "" {
		return users[0].Username, nil
	}
	index, convErr := strconv.Atoi(choice)
	if convErr != nil || index < 1 || index > len(users) {
		// Accept a username too, which is friendlier than re-running.
		for _, u := range users {
			if u.Username == choice {
				return u.Username, nil
			}
		}
		return "", fmt.Errorf("invalid selection %q", choice)
	}
	return users[index-1].Username, nil
}

func userExists(users []sourceUser, username string) bool {
	for _, u := range users {
		if u.Username == username {
			return true
		}
	}
	return false
}

func userNames(users []sourceUser) string {
	names := make([]string, 0, len(users))
	for _, u := range users {
		names = append(names, u.Username)
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

func rulesOf(all []externalRule, sourceId string) []externalRule {
	out := make([]externalRule, 0)
	for _, e := range all {
		if e.SourceId == sourceId {
			out = append(out, e)
		}
	}
	return out
}

func normalizeStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "SUCCESS", "OK":
		return "SUCCESS"
	case "FAILED":
		return "FAILED"
	default:
		return "NEVER"
	}
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func loadManualRules(db *sql.DB, username string) ([]manualRule, error) {
	query := `SELECT m.type, m.value, m.policy, m.note, m.enabled, m.sortOrder
	          FROM ManualRule m JOIN User u ON u.id = m.userId`
	args := []any{}
	if username != "" {
		query += ` WHERE u.username = ?`
		args = append(args, username)
	}
	query += ` ORDER BY m.sortOrder, m.id`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []manualRule
	for rows.Next() {
		var r manualRule
		var enabled any
		if err := rows.Scan(&r.Type, &r.Value, &r.Policy, &r.Note, &enabled, &r.SortOrder); err != nil {
			return nil, err
		}
		r.Enabled = boolFromAny(enabled)
		out = append(out, r)
	}
	return out, rows.Err()
}

func loadSources(db *sql.DB, username string) ([]source, error) {
	query := `SELECT s.id, s.name, s.url, s.defaultPolicy, s.detectedFormat, s.interval, s.enabled,
	                 s.sortOrder, s.lastSyncedAt, s.lastStatus, s.lastError, s.ruleCount, s.userId
	          FROM ExternalSource s JOIN User u ON u.id = s.userId`
	args := []any{}
	if username != "" {
		query += ` WHERE u.username = ?`
		args = append(args, username)
	}
	query += ` ORDER BY s.sortOrder, s.id`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []source
	for rows.Next() {
		var s source
		var enabled any
		if err := rows.Scan(&s.Id, &s.Name, &s.Url, &s.DefaultPolicy, &s.DetectedFormat, &s.Interval,
			&enabled, &s.SortOrder, &s.LastSyncedAt, &s.LastStatus, &s.LastError, &s.RuleCount, &s.UserId); err != nil {
			return nil, err
		}
		s.Enabled = boolFromAny(enabled)
		out = append(out, s)
	}
	return out, rows.Err()
}

func loadExternalRules(db *sql.DB, username string) ([]externalRule, error) {
	query := `SELECT e.sourceId, e.type, e.value, e.policy, e.enabled, e.sourceOrder, e.raw
	          FROM ExternalRule e JOIN User u ON u.id = e.userId`
	args := []any{}
	if username != "" {
		query += ` WHERE u.username = ?`
		args = append(args, username)
	}
	query += ` ORDER BY e.sourceId, e.sourceOrder, e.id`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []externalRule
	for rows.Next() {
		var e externalRule
		var enabled any
		if err := rows.Scan(&e.SourceId, &e.Type, &e.Value, &e.Policy, &enabled, &e.SourceOrder, &e.Raw); err != nil {
			return nil, err
		}
		e.Enabled = boolFromAny(enabled)
		out = append(out, e)
	}
	return out, rows.Err()
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
