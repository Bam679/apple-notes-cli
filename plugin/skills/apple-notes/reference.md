# apple-notes reference

## Fields
- notes: `id, title, folder, folder_id, created, modified, locked, attachments, text (get), html (get --html)`
- folders: `id, path, name, account, parent_id, notes`

`id` is the Notes Core Data URI (`x-coredata://…/ICNote/p123`). It is stable and can be passed back to `get`, `append`, `update`, `move` and `delete`.

## Commands
| command | policy op | notes |
|---|---|---|
| `accounts`, `folders [--account]` | read | read-denied folders are omitted |
| `list [--folder]… [--modified-after D] [--modified-before D]` | read | newest first, no bodies |
| `search <text> [--folder]… [--body]` | read | case-insensitive |
| `get <id> [--html]` | read | exit 4 if the folder is read-denied |
| `add` | add | `--folder --title [--body]` or `-` with JSON objects `{folder,title,body}` |
| `append <id> --body` | append | adds paragraphs at the end |
| `update <id> --body [--title]` | update | replaces content; the title is kept unless `--title` |
| `move <id>… --to F` | move | write scope is needed on the source and target folders |
| `delete <id>…` | delete | moves notes to Recently Deleted |
| `folder-create <Account/Path>` | folder-create | the parent must exist |
| `folder-delete <folder>` | folder-delete | only empty folders |

Folders can be referenced by full path (`iCloud/Work/Projects`), id, or name when it is unique.

## Policy (user-owned, read-only for agents)
`~/.config/agent-cli/apple-notes.json`. Scopes match **folder paths**. Example: read everything except `iCloud/Private`, and write only below `iCloud/Agent`:
```json
{"version":1,"operations":{"allow":["read","add","append"]},"scopes":{"read":{"allow":["*"],"deny":["iCloud/Private*"]},"write":{"allow":["iCloud/Agent","iCloud/Agent/*"]}}}
```
