# apple-notes

Read, search and write Apple Notes from the command line, built for AI coding agents.

[![ci](https://github.com/Bam679/apple-notes-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/Bam679/apple-notes-cli/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/Bam679/apple-notes-cli)](https://github.com/Bam679/apple-notes-cli/releases)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/Bam679/apple-notes-cli/badge)](https://scorecard.dev/viewer/?uri=github.com/Bam679/apple-notes-cli)
[![licence](https://img.shields.io/badge/licence-MIT-blue.svg)](LICENSE)

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/logo-dark.svg">
  <img alt="apple-notes logo: the agent-cli bracket mark with the apple-notes glyph" src="assets/logo.svg" height="56">
</picture>

`apple-notes` controls the Notes app through fixed JXA scripts and prints one line per note, or a note's plain text. Everything runs on your Mac, and the CLI itself makes no network connections. Writes stay off until a policy file allows them, and password-protected notes are never read.

Measured on the 20 most recently changed notes (2026-11-20, v0.1.0, macOS 26.6.2): the default text output was 3,211 bytes and `--json` 7,052 bytes. The command took 3.6 s, because Notes reads every folder for a newest-first list. Bytes are not tokens; `apple-notes list --limit 20 | wc -c` shows your own number.

## Why a CLI instead of an MCP server

An MCP server loads its tool definitions into the agent's context in every session, and each result travels back through the model before the agent can act on it. A CLI costs nothing until it is called. The agent learns it from a short skill, filters the output with `jq` or `grep`, and chains several steps in one shell script, for example turning this week's events into reminders in a single run.

`apple-notes` is one of five CLIs with the same output format, exit codes and permission model: [apple-mail](https://github.com/Bam679/apple-mail-cli), [apple-notes](https://github.com/Bam679/apple-notes-cli), [apple-calendar](https://github.com/Bam679/apple-calendar-cli), [apple-reminders](https://github.com/Bam679/apple-reminders-cli) and [paperless](https://github.com/Bam679/paperless-cli). Together they let an agent manage everyday things on a Mac with one set of rules. If your agent cannot run shell commands, an MCP server is the better fit.

## Install

1. Install the CLI:

   ```sh
   brew install Bam679/tap/apple-notes
   ```

   Signed release archives and `go install` are described in [How to install and verify a release](docs/how-to/install-and-verify.md).

2. Check that it can reach Notes:

   ```sh
   apple-notes doctor
   ```

   On the first run macOS asks whether your terminal app may control Notes. See [Security and permissions](#security-and-permissions).

3. Connect Claude Code. The plugin adds a skill that teaches the agent the CLI, and a hook that stops the agent from editing the policy file:

   ```sh
   claude plugin marketplace add Bam679/apple-notes-cli
   claude plugin install apple-notes@apple-notes-cli
   ```

   To get all five tools and the shared `agent-cli` skill from one place, use the [agent-cli-plugins](https://github.com/Bam679/agent-cli-plugins) marketplace instead. Install each plugin from one marketplace only.

To remove everything: `claude plugin uninstall apple-notes`, `brew uninstall apple-notes`, and delete `~/.config/agent-cli/apple-notes.json` if you created one.

## Quick start

```sh
apple-notes doctor
```

```text
notes_automation	true	28 folders visible
```

```sh
apple-notes folders                                   # one line per folder: id, path, number of notes
apple-notes search "router" --body --fields id,title  # titles and note text
apple-notes get <id> --max-chars 2000                 # one note as plain text, capped
```

Without a policy file every write is refused, a dry run included:

```sh
apple-notes add --folder "iCloud/Agent" --title "Test" --dry-run
```

```text
{"error":{"code":"policy_denied","message":"operation \"add\" is not allowed by policy","hint":"the policy file must allow this; agents must not edit it","exit":4}}
```

The [tutorial](docs/tutorial/getting-started.md) continues from here to a first write in a container made for the exercise.

## Use it with your agent

With the plugin installed, Claude Code picks up the skill on its own. For other agents (Codex, Cursor, Gemini CLI), add this to `AGENTS.md`:

```markdown
Use `apple-notes` for the user's Apple Notes. Run `apple-notes schema` once to learn the commands.
Writes need the user's policy file: preview with `--dry-run`, then run with `--confirm`
only after the user agreed. On exit code 4, stop and tell the user.
```

What an agent can rely on: the default output has one item per line with tab-separated fields; `--json` prints `{"items":[…],"count":N,"next_cursor":"…"}` and `--ndjson` one object per line. `--fields`, `--limit` (default 50, at most 1000) and `--max-chars` keep output small, and `--cursor` continues a list. Errors are one JSON line on stderr, and exit codes are fixed:

| Exit | Meaning |
|---|---|
| 0 | success |
| 1 | unexpected error |
| 2 | invalid arguments or flags |
| 3 | item or container not found |
| 4 | denied by the policy; no flag overrides this |
| 5 | write without `--confirm`; the plan is printed, nothing changed |
| 6 | missing macOS permission or credentials |
| 7 | backend did not answer in time |
| 8 | ambiguous name, or the item already exists |

The [CLI reference](docs/reference/cli.md) has every command, flag, field and the JSON shapes.

## How it works

`apple-notes` runs one fixed JXA script through `/usr/bin/osascript`, which controls Notes with Apple Events. Your input reaches the script as a single JSON argument, and note text is HTML-escaped in Go before it becomes note content, so text from you or the agent never becomes code. Every write command first builds a plan. The binary checks the plan against the policy file, then prints it (`--dry-run`, or no `--confirm`) or applies it (`--confirm`). [Security model](docs/explanation/security-model.md) explains why the checks live in the binary.

## When not to use it

- You need checklists, highlights, block quotes or clickable link text in new notes. The Notes scripting interface drops them (tested on macOS 26.6.2). Headings, text styles, colours, font sizes, lists, tables, code blocks and attachments work through `--markdown` and `--attach`.
- You want to search thousands of long notes often. `search --body` reads every note's text through Apple Events and takes seconds per run.
- Your agent cannot run shell commands. An MCP server fits better there.

## Security and permissions

`apple-notes` only reads until you allow writes in `~/.config/agent-cli/apple-notes.json`:

```json
   {
     "version": 1,
     "operations": {
       "allow": [
         "read",
         "add",
         "append"
       ],
       "deny": [
         "delete",
         "folder-delete"
       ]
     },
     "scopes": {
       "read": {
         "allow": [
           "*"
         ],
         "deny": [
           "iCloud/Private*"
         ]
       },
       "write": {
         "allow": [
           "iCloud/Agent*"
         ],
         "deny": []
       }
     },
     "limits": {
       "max_write_batch": 10
     }
   }
```

Operations are `read`, `write` (all writes) or single operations: `add`, `append`, `update`, `move`, `delete`, `folder-create`, `folder-delete`. Scopes are glob patterns over folder paths `Account/Folder/Sub` (e.g. `iCloud/Agent*`). Deny wins over allow, and an empty `scopes.read.allow` means everything may be read. `apple-notes policy` shows what is allowed now.

The binary reads the policy from that fixed path only. It takes the home directory from the user database, not from `$HOME`, and no flag or environment variable points it elsewhere. A file that is invalid, writable by other users or owned by someone else makes every command fail with exit code 4.

`apple-notes policy --claude-settings` prints Claude Code settings that allow read commands, ask before every write, deny access to the policy directory, and turn on the Bash sandbox with only `apple-notes` excluded. In the sandbox the agent cannot reach Notes through `osascript`, the network or the policy file directly, so `apple-notes` and its policy are the only way to the data. [How to configure permissions](docs/how-to/configure-permissions.md) walks through it.

### macOS privacy permission

`apple-notes` controls Notes.app with Apple Events through `/usr/bin/osascript`. macOS asks on first use whether the app that runs the CLI (Terminal, iTerm, VS Code or Claude) may control Notes. If you declined, open System Settings > Privacy & Security > Automation, find your terminal app and switch on Notes. Full Disk Access is not needed. `apple-notes doctor` shows whether access works; without it, commands exit with code 6.

### What leaves your machine

`apple-notes` makes no network connections. Notes stay in the Notes app, and iCloud syncs them as usual. There is no telemetry and no update check. Whatever an agent reads through `apple-notes` is sent to the model provider by the agent, like any other tool output.

### Threat model

| Threat | Mitigation |
|---|---|
| A prompt-injected agent tries destructive writes | Writes are off by default. The binary checks an operation and scope allowlist, needs `--confirm` and limits batch sizes. |
| The agent edits its own permissions | The policy path is fixed. The plugin hook and the recommended deny rules block edits, and the sandbox blocks writes outside the project. |
| Input turns into code | Static JXA script (`go:embed`). Input travels only as one JSON argv element to `/usr/bin/osascript` (absolute path), and note text is HTML-escaped in Go before it becomes note HTML. Fuzz tests check the script and argv shape are invariant, and an integration test round-trips hostile strings through real Notes. |
| Text in the data steers the agent | Output is data only and control characters are stripped. The skill tells agents to treat returned content as untrusted, and read scopes can hide containers. |
| Secrets or personal data leak | No credentials involved; macOS Automation permission controls access. Password-protected notes are never read. Errors never contain credentials. |
| Supply chain | No third-party Go dependencies. Actions are pinned to commit SHAs. CI runs govulncheck, gosec, CodeQL, gitleaks, dependency review and OpenSSF Scorecard. Releases ship signed checksums, SBOMs and build provenance. |

An agent with unrestricted shell access can go around any CLI, for example by calling `osascript` or `curl` directly. Only the sandbox closes that gap.

### Protecting PII with AI agents

- Hide sensitive containers with `scopes.read.deny`. Hidden containers never appear in the output.
- Ask for less. `--fields`, `--limit` and `--max-chars` keep personal data out of the context, and the skill tells agents to use them.
- Mask personal data before it reaches the model. [PasqualePillitteri/claude-privacy-tool](https://github.com/PasqualePillitteri/claude-privacy-tool) adds an offline masking hook to Claude Code, and [gregmos/PII-Shield](https://github.com/gregmos/PII-Shield) anonymises PII through an MCP server, a skill or a CLI.
- Scan tool output with a `PostToolUse` hook: [gitleaks/gitleaks](https://github.com/gitleaks/gitleaks) and [trufflesecurity/trufflehog](https://github.com/trufflesecurity/trufflehog) find secrets, and [data-privacy-stack/presidio](https://github.com/data-privacy-stack/presidio) (formerly microsoft/presidio) finds names, addresses, IBANs and phone numbers.
- Keep real data out of bug reports. Redact output or use synthetic examples.

## Commands

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

## Documentation

| Page | Use it when |
|---|---|
| [Getting started](docs/tutorial/getting-started.md) | you are new and want a first read and a first write |
| [How to configure permissions](docs/how-to/configure-permissions.md) | you want an agent to write, or to hide private data |
| [How to install and verify a release](docs/how-to/install-and-verify.md) | you install without Homebrew or want to check signatures |
| [CLI reference](docs/reference/cli.md) | you need a flag, a field, an exit code or the policy format |
| [Security model](docs/explanation/security-model.md) | you want to know why the checks work this way and where they end |
| [Architecture decisions](docs/adr) | you want the reasoning behind the design |

## Contributing

[CONTRIBUTING.md](CONTRIBUTING.md) has the rules for changes, and [SECURITY.md](SECURITY.md) explains how to report a vulnerability privately.

```sh
git clone https://github.com/Bam679/apple-notes-cli && cd apple-notes-cli
lefthook install   # gofmt, gitleaks and commit-message checks
make check         # everything CI runs
make integration   # tests against the real backend on this machine
```

## Licence

MIT. See [LICENSE](LICENSE).
