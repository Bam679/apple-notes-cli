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
