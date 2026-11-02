# rules-migrate

Copies routing rules from a [rules-manager](https://github.com/) v3 SQLite
database into the 3x-ui panel database.

## Usage

```bash
go build -o /tmp/rules-migrate ./scripts/rules-migrate

# 1. see who owns rules in the source database
/tmp/rules-migrate -source /path/to/rules-manager/prisma/dev.db -list-users

# 2. import, choosing the owner interactively
/tmp/rules-migrate -source /path/to/rules-manager/prisma/dev.db -target /etc/x-ui/x-ui.db

# 3. or name the owner directly (required when stdin is not a TTY)
/tmp/rules-migrate -source ... -target ... -user alice
```

Without `-user` the script lists the rules-manager accounts with their rule
counts and asks which one to import. A single-account source is imported
without asking; piped or empty stdin is refused instead of guessing.

Options:

| Flag | Meaning |
| --- | --- |
| `-source` | rules-manager SQLite file (opened read-only) |
| `-target` | 3x-ui `x-ui.db`; the script runs AutoMigrate first |
| `-user` | import only this rules-manager username (skips the prompt) |
| `-list-users` | print source accounts with rule counts and exit |
| `-dry-run` | print what would be imported, write nothing |
| `-skip-sources` | import manual rules only |
| `-replace` | clear existing manual rules and sources before importing |

Always run `-dry-run` first and keep a backup of the target database.

## What is converted

- `ManualRule` → `rules`. The type is normalized: rules-manager stores
  `DOMAIN_SUFFIX` / `IP_CIDR`, 3x-ui stores `DOMAIN-SUFFIX` / `IP-CIDR`.
- `ExternalSource` → `rule_sources`, keeping name, URL, default policy, sync
  interval, enabled flag, order and last sync status.
- `ExternalRule` → `rule_source_entries`, keeping source order and the raw line.
  Entries that fail type/policy validation, and duplicates of
  `(source, type, value, policy)`, are dropped.

3x-ui rules are global, so one import carries exactly one rules-manager
account's rule set. Sources with several accounts must be imported one at a
time — pick a different user, or run with `-replace` to swap sets.

Timestamps are rewritten to the import time; sync logs, users, sessions, node
pools and audit logs are not migrated.
