# Backlog

Small known issues, found in the review of phase 1a and left for later. Plans and decisions live in [DESIGN.md](DESIGN.md).

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

## zigbeat serve

- **"Ctrl-C again" after a Home Assistant OFF.** `Play` prints `stopping: interrupted (Ctrl-C again to quit without restoring)` also when the switch stopped the show; then no Ctrl-C happened and the next one only stops serve after the restore. Let the caller supply the hint.
- **ON during a restore that OFF did not start.** After a third panic the watcher in `server.play` (`internal/serve/lifecycle.go`) keeps reading `cmds` until `Play` returns, so an `ON` during that restore is dropped. Stop watching in the `Stopping` hook.
- **A brief stopping → running flip.** An `OFF` that lands between the end of the start and the first `Status` call, or on a 10 s tick, publishes `stopping`, `running`, `stopping`.
- **PID 1 in a container.** The kernel ignores default-action signals for PID 1, so a second signal does not quit there. The phase 4 image needs an init (`docker run --init`, `init: true`).

## Docs and code style

- A `ponytail:` prefix in a comment in `internal/run/show/kick.go`.

## Separate task

- **Clean up a failed mqtt subscribe.** In `internal/mqtt/subscribe.go`, a write error, a SUBACK timeout or a refused subscription leaves `c.acks[id]` and the handler in `c.subs` behind. Remove both on every failure path. Test it first with a minimal broker on `net.Pipe` or a local listener that never answers the SUBSCRIBE, or answers it with 0x80.
