# Backlog

Small known issues, found in the review of phase 1a and left for later. Plans and decisions live in [DESIGN.md](DESIGN.md).

## zigbeat run

- **Stale time after a reconnect.** While `mqttLink.Reconnect` blocks, both tickers in `play` (`internal/run/loop.go`) buffer one old tick. The next status line then shows a time from before the outage, and `Engine.Due` runs with that old `now`, which can raise a false "canary answered after 5xxx ms" signal. Use `time.Now()` instead of the channel values.
- **A silent broker drop leaves every light alone.** `Engine.Disconnected` keeps `fails` and `away`. A drop that is only detected by the keepalive (up to 30 s) gives every light two losses, so after the reconnect the lights only come back through the retry read a minute later. When ctx wins the select race in `play`, `Disconnected` is not called at all and the stop waits 2 s for echoes that cannot come.
- **`started` grows all night.** `Engine.started` (`internal/run/engine/`) gets one entry per unit and is never cleared. Delete the entry when the last message of a unit leaves or the unit is dropped.
- **A pause freezes lights mid-effect.** A pause, a light left alone or an outage leaves breathe lights moving until 100% or their minimum, and colour lights can stay at the kick level because the queued return is cleared. Spec decision: may a pause send one stop or anchor per breathe light?
- **"dropped" counts units.** The status line counts dropped units (`dropLate` in `internal/run/engine/queue.go`), while the README calls it "moments skipped".
- **No stack trace after a panic.** The recover in `play` prints only the panic value while it asks for a bug report. Print `debug.Stack()` to stderr.
- **Restore order.** Lights are restored in config order, so a light that is left alone (9 s of tries) delays the reachable lights behind it. Restore the lights that answered recently first.
- **The first redial at stop waits 5 s**, also when the broker is already back (`mqttLink.Reconnect`).
- **Timeouts shorter than the loss rule.** The 2 s drain and the 3 s tries in `lights.Snapshot` and `lights.Restore` are shorter than the 5 s after which a message counts as lost, so a slow bulb can get a second command before the first one was confirmed.
- **Snapshot pacing undercounts.** A snapshot read asks for 4 attributes, about 3 ZCL reads, but is paced as 1 frame.
- **Retained states after a re-subscribe** briefly bring lights that were left alone back into the show. They are left alone again after two misses.

## Docs and code style

- The cadence table in DESIGN.md: at exactly 30 BPM a moment falls on every beat, not every 2 beats.
- A `ponytail:` prefix in a comment in `internal/run/show/kick.go`.

## Separate task

- **Clean up a failed mqtt subscribe.** In `internal/mqtt/subscribe.go`, a write error, a SUBACK timeout or a refused subscription leaves `c.acks[id]` and the handler in `c.subs` behind. Remove both on every failure path. Test it first with a minimal broker on `net.Pipe` or a local listener that never answers the SUBSCRIBE, or answers it with 0x80.
