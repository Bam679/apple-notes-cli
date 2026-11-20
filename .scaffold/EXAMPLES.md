```sh
apple-notes folders                                     # id  path  notes
apple-notes list --folder iCloud/Notes --limit 10       # newest first
apple-notes search "wifi password" --body --json --fields id,title,folder
apple-notes get "x-coredata://…/ICNote/p123" --max-chars 4000
apple-notes add --folder iCloud/Agent --title "Standup" --body $'- done X\n- next Y' --dry-run
apple-notes append "x-coredata://…/ICNote/p123" --body "Follow-up: …" --confirm
printf '%s\n' '{"folder":"iCloud/Agent","title":"Idea","body":"…"}' | apple-notes add - --confirm
```
