
## Setup

1. Copy to `~/.config/systemd/user`
2. Reload `systemctl --user daemon-reload`
3. Enable `systemctl --user enable blueclip.service`
4. Monitor `systemctl --user status blueclip.service`
5. Logs `journalctl --user  -u blueclip `