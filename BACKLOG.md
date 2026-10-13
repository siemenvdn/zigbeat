# Backlog

Small known issues, found in the reviews of phases 1a to 3c and of the cleanup, in the first session on real hardware and while building phase 4, and left for later. Plans and decisions live in [DESIGN.md](DESIGN.md).

## zigbeat run

- **A silent broker drop can still pause the show.** A drop that only the keepalive finds (up to about 35 s) gives the lights losses that raise backoff. After the reconnect their failure counts start over and lights left alone are read at once, but a pause holds those reads until the backoff steps down, up to a minute. Idea: a loss while the canary does not answer either is the network's, not the light's.
- **A pause can leave a colour light at its kick.** A pause clears the queue, also a kick's return, so a colour light can stay at the kick level until its next change. Breathe lights get their low end when the pause ends (`Planner.Settle`). Spec decision: may a pause send one return per colour light?
- **The canary read can wait behind a moment.** `Due` (`internal/run/engine/due.go`) reads the canary whenever an in-flight slot is free, so with one full moment in flight (six messages) the read can queue behind about 8 frames of zigbeat's own, and `z2m.CanaryLimit` then measures zigbeat, not the house. On fake-z2m (`-service 30ms`) it reached 271 ms against a limit of 345 ms. Watch the `canary` column in the tempo retest; on a `backoff 1: canary answered after ...` line, read the canary only when nothing of the show is in flight, with a fallback after `flow.CanaryEvery` so it cannot starve.
- **A late kick and a later one of the same light.** When a light's first colour change is dropped late and its next one later on its own, the second's colour from before is the first's, which never showed (`Planner.Missed`, `internal/run/show/colors.go`). Rare: two late drops in a row on one light.
- **"pace every bar again" for a pace that never played.** `noteStretch` (`internal/run/show/pace.go`) says `again` when the budget stops stretching, also when Home Assistant just chose the pace it was stretched to. Reset `stretched` when `SetEffects` changes the pace, or say "the budget no longer stretches the pace".
- **A `max_rate` below the canary's share.** `budgetRate` (`internal/run/show/pace.go`) subtracts the canary's 0.2 frames/s, so `max_rate: 0.2` or less leaves no room: colour still goes every 16 beats, but a breathe-only config plans nothing. The probe never writes such a budget; refuse it in `config` with a message.
- **Pace 1 under load.** At `every beat` the kick's return lands half a beat after the kick, so on a loaded network the next kick of the same light may wait for the return's confirmation and pass `lateLimit`: expect some `dropped`. Say in the README that every beat is for strong networks, or let the property test cover pace 1.
- **Album colours of a warm cover look alike.** A cover in orange, yellow and red gave `#ff9114 #f7ff14 #ff1c14` on real bulbs, hues about 30° apart: the least `AlbumColors` (`internal/run/show/palette.go`) allows, and nearly one colour in the room. A track without album colours looked much better. Idea: a wider least gap, or fill up with a far hue when the cover's hues lie close together.
- **A restore longer than the minute to stop.** Each light that does not answer takes `restoreTries` times `answerWait`, 15 s (`internal/lights/restore.go`), one after the other. With four or more of them the restore passes the minute of `stop_grace_period`, and the kill cuts off their `could not confirm` lines; the lights that answer are back already, because lights the show left alone come last. Idea: one try for a light the show left alone.
- **Five album colours for six.** `AlbumColors` (`internal/run/show/palette.go`) fills up with the dominant hue ±30°, ±60° and ±90°. A cover with hues between those steps blocks two of them each: red, amber and pink (0°, 45°, 315°) with `album_colors: 6` give five colours. Add ±120°, ±150° and 180° to `fillSteps`; then six always fit.

## Music Assistant source and fake-ma

- **fake-ma sends a new track's waveform before its clear.** `play` (`cmd/fake-ma/timeline.go`) starts the new music before `clear`, so `waveform` can send the first 5 s of the next track while the old one still plays; zigbeat's meter then drops them at the clear. Music Assistant clears first. Call `music` after `clear`.

## zigbeat serve

- **Breathe low and high arrive one at a time.** Each slider is checked with the other end in force (`Choose`, `internal/serve/ha/style.go`), so after a restart the broker's replay order decides: with the config at [10, 100] and the choices low 2, high 5, a `breathe_high` that arrives first is refused ([10, 5]) and the show plays [2, 100]. The spec refuses low not below high; moving the other end along, as Home Assistant's range sliders do, would keep the choice. Spec decision.
- **The same state twice.** At some switches serve publishes the same `zigbeat/state` twice in a row: `off` before `starting`, `stopping` twice, `off` twice. Harmless; skip a publish equal to the last one.
- **Kept choices print before the banner.** At a start with retained style choices, the `..., set in Home Assistant` lines come before `zigbeat serve: ...`.
- **"waiting for the switch" after a stop signal.** `server.loop` (`internal/serve/lifecycle.go`) prints it once more before it sees the done context, so a container's log ends with it.
- **The quit hint on SIGTERM.** The stop line says `Ctrl-C again to quit without restoring` (`cli.QuitHint`) also after a SIGTERM, which a container never gets from Ctrl-C.

## zigbeat setup

- **The Music Assistant hint names a host.** The config setup writes suggests `ws://music-assistant:8095` (`internal/setup/write.go`); with the compose example's host network it is `ws://127.0.0.1:8095`, as in `config.example.yaml` and the README.
