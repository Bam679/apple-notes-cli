| Command | Kind | Description |
|---|---|---|
| `accounts` | read | Notes accounts |
| `folders [--account A]` | read | folders with full paths `Account/Folder/Sub` and note counts |
| `list [--folder F]… [--modified-after D] [--modified-before D]` | read | notes, newest first, without bodies |
| `search <text> [--folder F]… [--body]` | read | title match, or with `--body` also plain-text content |
| `get <id> [--html]` | read | one note as plain text (locked notes: metadata only) |
| `doctor` | read | permission check |
| `add --folder F --title T [--body B] [--markdown] [--attach FILE]…` / `add -` | write `add` | create notes, plain or formatted from Markdown, with attachments (stdin: JSON/NDJSON) |
| `append <id> --body B [--markdown] [--attach FILE]…` | write `append` | append paragraphs or formatted Markdown, and attachments |
| `update <id> --body B [--title T]` | write `update` | replace content |
| `move <id>… --to F` | write `move` | needs write scope on source and target |
| `delete <id>…` | write `delete` | moves to *Recently Deleted* |
| `folder-create <Account/Path>` / `folder-delete <folder>` | write | create / delete an empty folder |
| `schema`, `policy`, `guard`, `version` | builtin | contract, effective policy, hook guard, version |

Folders can be given as full path, id, or unique name. Without `--markdown`, bodies are plain text, HTML-escaped and stored as paragraphs.
