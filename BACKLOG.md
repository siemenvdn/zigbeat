# Backlog

Small known issues, found in the reviews of phases 1a to 3c and of the cleanup, and left for later. Plans and decisions live in [DESIGN.md](DESIGN.md).

## zigbeat run

- **A silent broker drop can still pause the show.** A drop that only the keepalive finds (up to about 35 s) gives the lights losses that raise backoff. After the reconnect their failure counts start over and lights left alone are read at once, but a pause holds those reads until the backoff steps down, up to a minute. Idea: a loss while the canary does not answer either is the network's, not the light's.
- **A pause freezes lights mid-effect.** During a backoff pause, a light left alone or an outage, breathe lights keep moving until 100% or their minimum, and colour lights can stay at the kick level because the queued return is cleared. Breathe lights get their low end when the pause ends (`Planner.Settle`); during it they still run free. Spec decision: may a pause send one stop or anchor per breathe light?
- **A cleared queue does not tell the planner.** A pause (`paused`), a dropped link (`Disconnected`) and a light left alone (`failed`) in `internal/run/engine` clear queued colour changes without `Planner.Missed`, so `build` takes those lights for done until the target moves on. Route them through `drop`, which tells the planner.
- **Five album colours for six.** `AlbumColors` (`internal/run/show/album.go`) fills up with the dominant hue ±30°, ±60° and ±90°. A cover with hues between those steps blocks two of them each: red, amber and pink (0°, 45°, 315°) with `album_colors: 6` give five colours. Add ±120°, ±150° and 180° to `fillSteps`; then six always fit.

## Music Assistant source and fake-ma

- **An empty waveform frame makes energy NaN.** `parse` (`internal/music/ma/message.go`) accepts a waveform frame without samples, and `meter.add` then divides 0 by 0, so every beat until the next clear has energy NaN. Music Assistant always sends 1024 samples; skip a frame without them.
- **fake-ma sends a new track's waveform before its clear.** `play` (`cmd/fake-ma/timeline.go`) starts the new music before `clear`, so `waveform` can send the first 5 s of the next track while the old one still plays; zigbeat's meter then drops them at the clear. Music Assistant clears first. Call `music` after `clear`.
- **`bar()` repeats the beat period.** `internal/music/ma/deliver.go` computes a bar from the tempo itself; `music.Track{BPM: ...}.Period()` already does.

## zigbeat serve

- **PID 1 in a container.** The kernel ignores default-action signals for PID 1, so a second signal does not quit there. The phase 4 image needs an init (`docker run --init`, `init: true`).
- **`ha.Value` panics on an unknown key.** `find` (`internal/serve/ha/style.go`) returns an empty setting and `Value` calls its nil `value`. serve calls it only after `Choose` accepted the key; return nil when the key is unknown.
- **serve prints on the MQTT reader.** `choose` (`internal/serve/style.go`) prints its lines on the reader goroutine, whose handlers must not block, so a stalled stdout pipe would stall the Home Assistant link. Idea: hand the line to `link.run`.

## zigbeat setup

- **The commented `effects` block misses the 3b keys.** `internal/setup/write.go` writes `cadence`, `brightness` and `breathe_range` as comments, not `album_palette`, `album_colors` and `variant`.
