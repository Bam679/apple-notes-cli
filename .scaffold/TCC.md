### macOS privacy permission

`apple-notes` controls Notes.app with Apple Events through `/usr/bin/osascript`. macOS asks on first use whether the app that runs the CLI (Terminal, iTerm, VS Code or Claude) may control Notes. If you declined, open System Settings > Privacy & Security > Automation, find your terminal app and switch on Notes. Full Disk Access is not needed. `apple-notes doctor` shows whether access works; without it, commands exit with code 6.
