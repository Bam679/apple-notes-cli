## macOS privacy (TCC)

`apple-notes` controls Notes.app via Apple Events (`/usr/bin/osascript`). On first use, macOS asks whether **your terminal app** (Terminal, iTerm, VS Code, Claude desktop …) may control **Notes**. If you denied it, enable it under **System Settings > Privacy & Security > Automation > [your terminal] > Notes**. No Full Disk Access is needed. `apple-notes doctor` reports the status, and errors use exit code 6.
