# MSC Multiplayer Server

Multiplayer server for **My Summer Car** game.  
Part of the **MSCMultiplayer** mod project by **Pyt_o**.

## Features
- UDP-based fast networking
- Up to 255 players per server
- Pterodactyl / Pelican Panel egg included
- Auto-cleanup of timed out players

## Hosting
Deploy instantly using the included egg on any Pterodactyl/Pelican panel.  
Default port: `7777 UDP`

## Building manually
```bash
go build -o mscmp-server main.go
./mscmp-server
```

## Environment variables
| Variable | Default | Description |
|----------|---------|-------------|
| SERVER_PORT | 7777 | UDP port |
| MAX_PLAYERS | 10 | Max players |

## License
GPL v3 — see LICENSE
