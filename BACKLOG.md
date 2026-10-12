# Backlog

Small known issues, found in the reviews of phases 1a to 2 and left for later. Plans and decisions live in [DESIGN.md](DESIGN.md).

## zigbeat run

- **A silent broker drop leaves every light alone.** `Engine.Disconnected` keeps `fails` and `away`. A drop that is only detected by the keepalive (up to 30 s) gives every light two losses, so after the reconnect the lights only come back through the retry read a minute later. When ctx wins the select race in `play`, `Disconnected` is not called at all and the stop waits 2 s for echoes that cannot come. Under `zigbeat serve` Home Assistant shows it: `lights` 0 and `lights_away` all lights for up to a minute.
- **`started` grows all night.** `Engine.started` (`internal/run/engine/`) gets one entry per unit and is never cleared. Delete the entry when the last message of a unit leaves or the unit is dropped.
- **A pause freezes lights mid-effect.** During a backoff pause, a light left alone or an outage, breathe lights keep moving until 100% or their minimum, and colour lights can stay at the kick level because the queued return is cleared. Breathe lights get their low end when the pause ends (`Planner.Settle`); during it they still run free. Spec decision: may a pause send one stop or anchor per breathe light?
- **"dropped" counts units.** The status line counts dropped units (`dropLate` in `internal/run/engine/queue.go`), while the README calls it "moments skipped".
- **Restore order.** Lights are restored in config order, so a light that is left alone (9 s of tries) delays the reachable lights behind it. Restore the lights that answered recently first.
- **The first redial at stop waits 5 s**, also when the broker is already back (`mqttLink.Reconnect`).
- **Timeouts shorter than the loss rule.** The 2 s drain and the 3 s tries in `lights.Snapshot` and `lights.Restore` are shorter than the 5 s after which a message counts as lost, so a slow bulb can get a second command before the first one was confirmed.
- **Snapshot pacing undercounts.** A snapshot read asks for 4 attributes, about 3 ZCL reads, but is paced as 1 frame.
- **Retained states after a re-subscribe** briefly bring lights that were left alone back into the show. They are left alone again after two misses.
- **The show's connect ignores ctx.** `Dial` and the subscriptions in `connect` (up to 5 s per step) do not watch ctx, so an `OFF` or Ctrl-C during a slow connect waits for it.
- **`+20` is absolute.** In the offset line `+` moves 10 ms later, but `+20` sets 20 ms, as the spec says. At 180 ms, `+20` jumps 160 ms earlier. Reject a signed number, or make it relative.

## Music Assistant

- **Bar numbering at a track start depends on a race.** When the first beat of a track is already less than 1 s ahead, `session.arm` resets the emit timer to a negative duration, so `deliver` races the rest of the burst. If the timer wins, a pickup beat is numbered 0 and the downbeat 4; if the messages win, −1 and 0 (seen in the e2e run of `zigbeat beats`: bar 0 where the plan's run had bar 1). At 8 beats per moment the moments then fall on odd bars instead of even ones, still on a bar line. Below about 67 BPM a pickup that lands 0.9 to 1 s ahead is delivered with Index 0, so a moment falls on the upbeat. Arm only once the burst is quiet (as `report` does), or drain pending messages before `deliver`.
- **Another player after the music stops.** Without `music_assistant.player`, the relay picks the playing player when zigbeat connects and stays with it. Music stopping on one speaker and starting on another leaves zigbeat silent until the connection drops or the show restarts. Reconnecting on `stream/end` when no player is set would follow, at the cost of up to 5 s after every resume.
- **Beats already handed over survive a seek.** Up to 1 s of beats sits in the engine when a `stream/clear` or `stream/end` comes, so one kick and its return can still land on the old grid. Inherent to the 1 s lookahead; an engine call to drop future units would fix it.

## zigbeat serve

- **"Ctrl-C again" after a Home Assistant OFF.** `Play` prints `stopping: interrupted (Ctrl-C again to quit without restoring)` also when the switch stopped the show; then no Ctrl-C happened and the next one only stops serve after the restore. Let the caller supply the hint.
- **ON during a restore that OFF did not start.** After a third panic the watcher in `server.play` (`internal/serve/lifecycle.go`) keeps reading `cmds` until `Play` returns, so an `ON` during that restore is dropped. Stop watching in the `Stopping` hook.
- **A brief stopping → running flip.** An `OFF` that lands between the end of the start and the first `Status` call, or on a 10 s tick, publishes `stopping`, `running`, `stopping`.
- **PID 1 in a container.** The kernel ignores default-action signals for PID 1, so a second signal does not quit there. The phase 4 image needs an init (`docker run --init`, `init: true`).

## Docs and code style

- A `ponytail:` prefix in a comment in `internal/run/show/kick.go`.

## Separate task

- **Clean up a failed mqtt subscribe.** In `internal/mqtt/subscribe.go`, a write error, a SUBACK timeout or a refused subscription leaves `c.acks[id]` and the handler in `c.subs` behind. Remove both on every failure path. Test it first with a minimal broker on `net.Pipe` or a local listener that never answers the SUBSCRIBE, or answers it with 0x80.
