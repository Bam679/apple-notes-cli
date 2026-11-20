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
