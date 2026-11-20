# Getting started with apple-notes

In this tutorial you install `apple-notes`, read some of your Apple Notes data with it, and then let it make one change in a container that exists only for this exercise. It takes about ten minutes. You need a terminal and macOS 14+, Apple Silicon and Intel.

## Install the CLI

```sh
brew install Bam679/tap/apple-notes
apple-notes version
```

## Give it access to Notes

```sh
apple-notes doctor
```

macOS asks whether your terminal may control Notes. Choose OK. The command prints a line like `notes_automation  true  28 folders visible`.

## Read your notes

```sh
apple-notes accounts
apple-notes folders
apple-notes list --limit 5
```

You see your accounts, every folder with its full path, and your five most recently changed notes. This tutorial assumes an account called `iCloud`. If `apple-notes accounts` shows another name, use that wherever you see `iCloud` below.

## Try to change something

```sh
apple-notes folder-create "iCloud/Agent Tutorial" --dry-run
```

The command exits with code 4: without a policy file the CLI only reads.

## Allow writes in one folder

```sh
mkdir -p ~/.config/agent-cli
cat > ~/.config/agent-cli/apple-notes.json <<'JSON'
{
  "version": 1,
  "operations": { "allow": ["read", "write"] },
  "scopes": { "write": { "allow": ["iCloud/Agent Tutorial"] } }
}
JSON
chmod 600 ~/.config/agent-cli/apple-notes.json
apple-notes policy
```

## Write a note

```sh
apple-notes folder-create "iCloud/Agent Tutorial" --confirm
apple-notes add --folder "iCloud/Agent Tutorial" --title "Shopping" --body "Bread"
```

The `add` command exits with code 5 and shows the note it would create. Run it with `--confirm`, then append a line and read the note back:

```sh
apple-notes add --folder "iCloud/Agent Tutorial" --title "Shopping" --body "Bread" --confirm
apple-notes list --folder "iCloud/Agent Tutorial" --fields id,title
apple-notes append <id> --body "Milk" --confirm
apple-notes get <id>
```

The note now has two lines, and it shows up in the Notes app.

## Clean up

```sh
apple-notes delete <id> --confirm
apple-notes folder-delete "iCloud/Agent Tutorial" --confirm
rm ~/.config/agent-cli/apple-notes.json
```

The deleted note stays in Recently Deleted for 30 days, as usual in Notes.

## What you have done

You installed `apple-notes` and read data with it. You saw that it refuses to write until a policy allows it, previewed a change with `--dry-run`, and applied it with `--confirm`.

Next steps:

- [How to configure permissions](../how-to/configure-permissions.md) when you want an agent to use the CLI
- [CLI reference](../reference/cli.md) for every command and flag
