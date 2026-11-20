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

## Formatting with --markdown

| Markdown | Result in Notes |
|---|---|
| `# `, `## `, `### ` | bold heading at 24 px, 18 px and body size |
| `**bold**`, `*italic*`, `~~strike~~` | bold, italic, strikethrough |
| `` `code` `` and fenced code blocks | Courier |
| `- item`, `1. item`, two-space indentation | bullet, numbered and nested lists |
| pipe tables with a `---` row | a Notes table; the first row is bold |
| `> quote` | italic paragraph (Notes has no quote style for scripts) |
| `<u>`, `<b>`, `<i>`, `<s>` | underline, bold, italic, strikethrough |
| `<font color="#E03131">`, `<font face="Georgia">` | text colour, font |
| `<span style="color: …; font-size: 28px; font-family: …">` | colour, size, font |

All other HTML is escaped and shows as text. Notes drops checklists, highlights, block quotes, alignment, indentation and link targets when a script writes a note, so the CLI cannot create them.

`--attach FILE` adds a regular file of up to 50 MiB, such as an image or PDF, as an attachment. Tables also count as attachments in the `attachments` field.

## Behaviour notes

- `--folder` and `--to` take a full path, an identifier, or a folder name when only one folder has that name.
- Bodies are plain text. The CLI escapes them and stores each line as a paragraph; `update` replaces the whole body, and `append` adds paragraphs at the end.
- `delete` moves notes to Recently Deleted. `folder-delete` only deletes empty folders.
- `search --body` reads every note's text in the selected folders, so it is slower than a title search.
