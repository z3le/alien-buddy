# alien-buddy

A friendly alien on an e-ink screen, teaching English.

## Hardware

- Raspberry Pi 2B (+ USB WiFi dongle)
- Waveshare 2.7" e-Paper HAT (264×176, black/white, SPI)

## Build

```bash
# On your dev machine (cross-compile for Pi 2B):
GOOS=linux GOARCH=arm GOARM=7 go build -o alien-buddy .

# Copy to Pi:
scp alien-buddy phrases.json pi@<pi-ip>:~/
```

## Run

```bash
# On the Pi — with real display:
./alien-buddy --phrases phrases.json --interval 2h

# Dev mode — no hardware, saves PNGs:
./alien-buddy --fake --phrases phrases.json --interval 10s

```

## Flags

```
--addr         listen address (default ":8080")
--phrases      path to phrases.json (default "phrases.json")
--interval     rotation interval (default "2h")
--fake         use fake display, saves PNGs to ./frames/
```

## API

```bash
# Push a message:
curl http://<pi-host>:8080/message \
  -H 'Content-Type: application/json' \
  -d '{"text": "Time for homework!", "mood": "stern"}'

# See what's on screen:
curl http://<pi-host>:8080/current

# Status:
curl http://<pi-host>:8080/
```

### Moods

`happy`, `stern`, `confused`, `sleepy`, `excited`, `thinking`

## How it works

1. Pi boots and starts an HTTP server on `--addr`
2. Rotates through phrases on a timer, drawing ASCII aliens + text to the e-ink display
3. Your phone can push messages via HTTP POST
4. Push messages pause rotation for 30 minutes, then rotation resumes

## Adding phrases

Edit `phrases.json`. Each entry:

```json
{
  "text": "Your phrase here",
  "category": "vocabulary",
  "mood": "happy"
}
```

Categories: `morning`, `day`, `evening`

## License

MIT — see [LICENSE](LICENSE).
