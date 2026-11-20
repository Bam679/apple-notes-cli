# apple-notes

Read, search and write Apple Notes from the command line, built for AI coding agents.

[![ci](https://github.com/Bam679/apple-notes-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/Bam679/apple-notes-cli/actions/workflows/ci.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/Bam679/apple-notes-cli/badge)](https://scorecard.dev/viewer/?uri=github.com/Bam679/apple-notes-cli)
[![release](https://img.shields.io/github/v/release/Bam679/apple-notes-cli)](https://github.com/Bam679/apple-notes-cli/releases)
[![licence](https://img.shields.io/badge/licence-MIT-blue.svg)](LICENSE)

## About

`apple-notes` lets an AI agent look things up in your notes and write meeting notes or summaries back. It controls Notes.app through static JXA scripts, prints one compact line per note, and stays read-only until you allow writes in a policy file. The checks run inside the binary, so they apply to every agent, whether that is Claude Code, Codex, Cursor or a script.

`apple-notes` is one of five CLIs that share the same output format, exit codes and permission model: [apple-mail](https://github.com/Bam679/apple-mail-cli), [apple-notes](https://github.com/Bam679/apple-notes-cli), [apple-calendar](https://github.com/Bam679/apple-calendar-cli), [apple-reminders](https://github.com/Bam679/apple-reminders-cli) and [paperless](https://github.com/Bam679/paperless-cli). All of them are available from one Claude Code marketplace, [agent-cli-plugins](https://github.com/Bam679/agent-cli-plugins).

## Why a CLI instead of an MCP server

There are MCP servers for some Apple apps, but each tool call goes back through the model, and the tool definitions fill the context before the agent has done anything. With a CLI, the agent learns the commands from a short skill, filters the output with `jq` or `grep`, and chains several steps in one script, for example turning this week's events into reminders in a single run. That is faster and uses far fewer tokens. Single-purpose CLIs exist for some of these apps, but not as a set with one output format and one permission model for Mail, Notes, Calendar, Reminders and Paperless-ngx, which is what you need to let Claude manage everyday things on a Mac.

## Features

- Folders with full paths, notes sorted by last change, and search in titles or full text
- Plain-text note bodies with a size limit, so long notes don't flood the agent's context
- Creating notes, appending to them, replacing their content and moving them, with text safely escaped
- Writes stay off until a policy file allows them, per operation and per folder
- Password-protected notes are never read or changed
- A Claude Code plugin with a skill for agents and a hook that protects the policy file

## Installation

Requirements: macOS 14+, Apple Silicon and Intel.

With Homebrew:

```sh
brew install Bam679/tap/apple-notes
```

From a release archive. Every release has signed checksums and SLSA build provenance, so check both before you install:

```sh
V=0.1.0 OS=darwin ARCH=arm64
base=https://github.com/Bam679/apple-notes-cli/releases/download/v$V
curl -fsSLO "$base/apple-notes_${V}_${OS}_${ARCH}.tar.gz"
curl -fsSLO "$base/checksums.txt"
curl -fsSLO "$base/checksums.txt.sigstore.json"
cosign verify-blob --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/Bam679/apple-notes-cli/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com checksums.txt
shasum -a 256 --ignore-missing -c checksums.txt
gh attestation verify "apple-notes_${V}_${OS}_${ARCH}.tar.gz" --repo Bam679/apple-notes-cli
tar xzf "apple-notes_${V}_${OS}_${ARCH}.tar.gz" apple-notes && install -m 755 apple-notes ~/.local/bin/
```

From source: `go install github.com/Bam679/apple-notes-cli/cmd/apple-notes@latest`

### Claude Code plugin

The plugin adds a skill that teaches agents the CLI, plus a hook that stops agents from editing the policy file. Install it from one marketplace only:

```sh
# this CLI on its own
claude plugin marketplace add Bam679/apple-notes-cli
claude plugin install apple-notes@apple-notes-cli

# or the whole family, including the shared agent-cli skill
claude plugin marketplace add Bam679/agent-cli-plugins
claude plugin install apple-notes@agent-cli-plugins
```

## Usage

```sh
apple-notes folders                                     # id  path  notes
apple-notes list --folder iCloud/Notes --limit 10       # newest first
apple-notes search "wifi password" --body --json --fields id,title,folder
apple-notes get "x-coredata://…/ICNote/p123" --max-chars 4000
apple-notes add --folder iCloud/Agent --title "Standup" --body $'- done X\n- next Y' --dry-run
apple-notes append "x-coredata://…/ICNote/p123" --body "Follow-up: …" --confirm
printf '%s\n' '{"folder":"iCloud/Agent","title":"Idea","body":"…"}' | apple-notes add - --confirm
```

New to the tool? The [getting started tutorial](docs/tutorial/getting-started.md) walks through a first read and a first write.

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

`apple-notes <command> --help` shows the flags of one command. `apple-notes schema` prints the whole contract as JSON: commands, flags, fields, exit codes and the policy format.

## Output contract

The default output has one item per line with tab-separated fields, no header and no colour. `--json` prints one document (`{"items":[…],"count":N,"next_cursor":"…"}` for lists), and `--ndjson` prints one object per line. Errors are a single JSON line on stderr, for example `{"error":{"code":"policy_denied","message":"…","hint":"…","exit":4}}`.

Agents keep their context small with `--fields`, `--limit` (default 50, at most 1000) and `--max-chars`. When a list continues, the output ends with `# next_cursor=…` (or a `next_cursor` key), and the same command with `--cursor` returns the next page.

| Exit | Name | Meaning |
|---|---|---|
| 0 | ok | success |
| 1 | internal | unexpected error |
| 2 | usage | invalid arguments or flags |
| 3 | not_found | the item or container does not exist |
| 4 | policy_denied | the policy file does not allow this; no flag overrides it |
| 5 | confirm_required | a write without `--confirm`; the plan is printed on stdout |
| 6 | permission_denied | missing macOS privacy permission or credentials |
| 7 | unavailable | the backend did not answer in time |
| 8 | conflict | an ambiguous name, or the item already exists |

The full description is in the [CLI reference](docs/reference/cli.md).

## Permissions

`apple-notes` only reads until you allow writes. A write goes through three checks inside the binary:

1. The policy file `~/.config/agent-cli/apple-notes.json` must allow the operation and every container it touches.
2. `--dry-run` shows the planned change without making it.
3. `--confirm` applies the change. Without it the command prints the plan and exits with code 5.

A policy that allows a few writes looks like this:

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

Scopes are glob patterns over folder paths `Account/Folder/Sub` (e.g. `iCloud/Agent*`). Deny rules win over allow rules, and an empty `scopes.read.allow` means everything may be read. The binary reads the file from a fixed path (the home directory comes from the user database, not `$HOME`), refuses files that other users can write, and treats a broken file as "deny everything". `apple-notes policy` shows what is currently allowed.

[How to configure permissions](docs/how-to/configure-permissions.md) covers the policy file, Claude Code permission rules and the recommended sandbox setup.

## macOS privacy permission

`apple-notes` controls Notes.app with Apple Events through `/usr/bin/osascript`. macOS asks on first use whether the app that runs the CLI (Terminal, iTerm, VS Code or Claude) may control Notes. If you declined, open System Settings > Privacy & Security > Automation, find your terminal app and switch on Notes. Full Disk Access is not needed. `apple-notes doctor` shows whether access works; without it, commands exit with code 6.

## Threat model

| Threat | Mitigation |
|---|---|
| A prompt-injected agent tries destructive writes | Writes are off by default. The binary checks an operation and scope allowlist, needs `--confirm` and limits batch sizes. |
| The agent edits its own permissions | The policy path is fixed. The plugin hook and the recommended deny rules block edits, and the Claude Code sandbox blocks writes outside the project. |
| Input turns into code | Static JXA script (`go:embed`). Input travels only as one JSON argv element to `/usr/bin/osascript` (absolute path), and note text is HTML-escaped in Go before it becomes note HTML. Fuzz tests check the script and argv shape are invariant, and an integration test round-trips hostile strings through real Notes. |
| Text in the data steers the agent | Output is data only and control characters are stripped. The skill tells agents to treat returned content as untrusted, and read scopes can hide containers. |
| Leaking secrets or personal data | No credentials involved; macOS Automation permission controls access. Password-protected notes are never read. There is no telemetry and no network traffic. Errors never contain credentials. |
| Supply chain | No third-party Go dependencies. Actions are pinned to commit SHAs. CI runs govulncheck, gosec, CodeQL, gitleaks, dependency review and OpenSSF Scorecard. Releases ship signed checksums, SBOMs and build provenance. |

The CLI cannot stop an agent that has unrestricted shell access from going around it, for example by calling `osascript` directly. The Claude Code sandbox closes that gap. [Security model](docs/explanation/security-model.md) explains the reasoning and the limits.

## Protecting PII with AI agents

Whatever an agent reads through this CLI is sent to the model provider. You can reduce that:

- Hide sensitive containers with `scopes.read.deny`. Hidden containers never appear in the output.
- Ask for less. `--fields`, `--limit` and `--max-chars` keep personal data out of the context, and the skill tells agents to use them.
- Mask personal data before it reaches the model. [PasqualePillitteri/claude-privacy-tool](https://github.com/PasqualePillitteri/claude-privacy-tool) adds an offline masking hook to Claude Code, and [gregmos/PII-Shield](https://github.com/gregmos/PII-Shield) anonymises PII through an MCP server, a skill or a CLI.
- Scan tool output with a Claude Code `PostToolUse` hook before it reaches the model. Tools that work well for this: [gitleaks/gitleaks](https://github.com/gitleaks/gitleaks) for secrets and tokens, [trufflesecurity/trufflehog](https://github.com/trufflesecurity/trufflehog) for verified secrets, and [data-privacy-stack/presidio](https://github.com/data-privacy-stack/presidio) (formerly microsoft/presidio) for names, email addresses, IBANs and phone numbers.
- Keep real data out of bug reports. Redact output or use synthetic examples.

## Documentation

- [Getting started](docs/tutorial/getting-started.md) (tutorial)
- [How to configure permissions](docs/how-to/configure-permissions.md)
- [CLI reference](docs/reference/cli.md)
- [Security model](docs/explanation/security-model.md)
- [Architecture decisions](docs/adr)

## Contributing

Contributions are welcome. [CONTRIBUTING.md](CONTRIBUTING.md) has the ground rules, and [SECURITY.md](SECURITY.md) explains how to report vulnerabilities privately.

```sh
git clone https://github.com/Bam679/apple-notes-cli && cd apple-notes-cli
lefthook install   # git hooks: gofmt, gitleaks, Conventional Commits
make check         # everything CI runs
make integration   # tests against real data in a sandbox (see CONTRIBUTING.md)
```

## Licence

`apple-notes` is licensed under the MIT Licence. See [LICENSE](LICENSE).
