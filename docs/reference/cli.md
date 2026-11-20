# apple-notes CLI reference

`apple-notes` version 0.x. `apple-notes schema` prints the same information as JSON and is always current for the installed version.

## Synopsis

```text
apple-notes <command> [arguments] [flags]
```

Flags can appear before or after arguments. Every flag has two dashes. `--` ends flag parsing.

## Commands

| Command | Kind | Description |
|---|---|---|
| `accounts` | read | Notes accounts |
| `folders [--account A]` | read | folders with full paths `Account/Folder/Sub` and note counts |
| `list [--folder F]… [--modified-after D] [--modified-before D]` | read | notes, newest first, without bodies |
| `search <text> [--folder F]… [--body]` | read | title match, or with `--body` also plain-text content |
| `get <id> [--html]` | read | one note as plain text (locked notes: metadata only) |
| `doctor` | read | permission check |
| `add --folder F --title T [--body B]` / `add -` | write `add` | create notes (stdin: JSON/NDJSON) |
| `append <id> --body B` | write `append` | append paragraphs |
| `update <id> --body B [--title T]` | write `update` | replace content |
| `move <id>… --to F` | write `move` | needs write scope on source and target |
| `delete <id>…` | write `delete` | moves to *Recently Deleted* |
| `folder-create <Account/Path>` / `folder-delete <folder>` | write | create / delete an empty folder |
| `schema`, `policy`, `guard`, `version` | builtin | contract, effective policy, hook guard, version |

Folders can be given as full path, id, or unique name. Bodies are plain text: they are HTML-escaped and stored as Notes paragraphs.

## Global flags

| Flag | Type | Description |
|---|---|---|
| `--json` | bool | Print one JSON document. |
| `--ndjson` | bool | Print one JSON object per line. |
| `--fields a,b` | string | Select and order fields. Unknown names are a usage error. |
| `--limit N` | int | Maximum number of items. Default 50, at most 1000. |
| `--cursor C` | string | Continue a list from a `next_cursor`. A cursor only works with the same command and flags. |
| `--max-chars N` | int | Cap the size of stdout. Lists drop whole items, single items shorten their longest text field. |
| `--dry-run` | bool | Writes only. Print the plan, change nothing. |
| `--confirm` | bool | Writes only. Apply the change. |
| `--timeout D` | duration | Abort after this time (default `60s`). |
| `--help` | bool | Show help. |

## Output formats

| Format | Lists | Single items |
|---|---|---|
| default | One line per item, tab-separated default fields, `-` for empty values. Trailer lines start with `#`: `# next_cursor=…`, `# truncated: …` | `field: value` lines |
| `--json` | `{"items":[…],"count":N,"next_cursor":"…","truncated":true}` | One object; `"truncated":true` when shortened |
| `--ndjson` | One object per line, then `{"_meta":{"next_cursor":…,"truncated":…}}` when relevant | One object |

Write commands print the changed items with an `op` field. With `--dry-run`, or without `--confirm`, they print the planned items with `"dry_run":true`.

Text output never contains control characters, tabs or newlines inside a value, or bidirectional override characters.

Errors are one JSON line on stderr:

```json
{"error":{"code":"policy_denied","message":"writing to \"Private\" is not allowed by policy","hint":"…","exit":4}}
```

## Exit codes

| Code | Name | Meaning |
|---|---|---|
| 0 | ok | Success. |
| 1 | internal | Unexpected error. |
| 2 | usage | Invalid arguments or flags. |
| 3 | not_found | The item or container does not exist. |
| 4 | policy_denied | The policy does not allow the operation or scope, or the policy file is invalid. No flag overrides this. |
| 5 | confirm_required | A write without `--confirm`. The plan is printed on stdout. |
| 6 | permission_denied | Missing macOS privacy permission, or invalid credentials. |
| 7 | unavailable | The backend did not answer, or the timeout expired. |
| 8 | conflict | An ambiguous name, or the item already exists. |

## Stdin

Write commands that take `-` as an argument read JSON objects from stdin: one object, an array of objects, or one object per line. Input is limited to 4 MiB, and unknown fields are rejected.

## Dates

Date flags accept `YYYY-MM-DD` (a whole day), `YYYY-MM-DDTHH:MM` and `YYYY-MM-DD HH:MM` in local time, RFC 3339, `today`, `tomorrow`, `yesterday`, `+Nd` and `-Nd`.

## Fields

| Field | Description |
|---|---|
| `id` | Notes Core Data URI such as `x-coredata://…/ICNote/p123`. Stable; pass it to `get`, `append`, `update`, `move` and `delete`. |
| `title` | The first line of the note. |
| `folder`, `folder_id` | Folder path (`Account/Folder/Sub`) and identifier. |
| `created`, `modified` | RFC 3339 timestamps. |
| `locked` | `true` for password-protected notes. Their content is never returned. |
| `attachments` | Number of attachments. |
| `text` | Plain-text body (`get` only). |
| `html` | HTML body (`get --html` only). |

Folder fields: `id`, `path`, `name`, `account`, `parent_id`, `notes` (number of notes).

The default text columns are `id modified title folder` for notes and `id path notes` for folders. Notes are sorted by last change, newest first.

## Scopes

Policy scopes match folder paths such as `iCloud/Work/Projects`. `move` needs write scope on both the source and the target folder.

## Behaviour notes

- `--folder` and `--to` take a full path, an identifier, or a folder name when only one folder has that name.
- Bodies are plain text. The CLI escapes them and stores each line as a paragraph; `update` replaces the whole body, and `append` adds paragraphs at the end.
- `delete` moves notes to Recently Deleted. `folder-delete` only deletes empty folders.
- `search --body` reads every note's text in the selected folders, so it is slower than a title search.

## Policy file

Path: `~/.config/agent-cli/apple-notes.json`. The home directory comes from the user database, not from `$HOME`. No flag or environment variable selects a different file.

| Key | Type | Meaning |
|---|---|---|
| `version` | int | Must be `1`. |
| `operations.allow` | string list | `read`, `write` (every write operation), or single operations: `add`, `append`, `update`, `move`, `delete`, `folder-create`, `folder-delete`. |
| `operations.deny` | string list | Denied operations. Deny wins over allow. |
| `scopes.read.allow` | glob list | Containers that may be read. Empty means all. |
| `scopes.read.deny` | glob list | Containers hidden from every read. |
| `scopes.write.allow` | glob list | Containers that may be changed. Empty means none. |
| `scopes.write.deny` | glob list | Containers that may never be changed. |
| `limits.max_write_batch` | int | Maximum number of items one write command may change (default 100). |

Scope strings are folder paths `Account/Folder/Sub` (e.g. `iCloud/Agent*`). In patterns `*` matches any text, including `/`; matching is case-sensitive.

Without a file, the default policy allows `read` on everything and no writes. The CLI rejects files that are not regular files, that are writable by group or others, that another user owns, or that contain unknown keys or invalid JSON. A rejected file denies every command except `schema`, `policy`, `version`, `help` and `guard`.

## Built-in commands

| Command | Description |
|---|---|
| `schema` | The machine-readable contract as JSON. |
| `policy` | The effective policy and its path. `--example` prints a template, and `--claude-settings` prints recommended Claude Code settings. |
| `guard` | Claude Code `PreToolUse` hook. It reads the hook JSON on stdin and denies tool calls that would modify agent-cli policy files. |
| `version` | Name and version. |
| `help` | Usage. |
