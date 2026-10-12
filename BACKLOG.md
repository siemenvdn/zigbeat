# Backlog

Small known issues, found in the reviews of phases 1a to 2 and left for later. Plans and decisions live in [DESIGN.md](DESIGN.md).

## zigbeat run

- **A silent broker drop leaves every light alone.** `Engine.Disconnected` keeps `fails` and `away`. A drop that is only detected by the keepalive (up to 30 s) gives every light two losses, so after the reconnect the lights only come back through the retry read a minute later. When ctx wins the select race in `play`, `Disconnected` is not called at all and the stop waits 2 s for echoes that cannot come. Under `zigbeat serve` Home Assistant shows it: `lights` 0 and `lights_away` all lights for up to a minute.
- **A pause freezes lights mid-effect.** During a backoff pause, a light left alone or an outage, breathe lights keep moving until 100% or their minimum, and colour lights can stay at the kick level because the queued return is cleared. Breathe lights get their low end when the pause ends (`Planner.Settle`); during it they still run free. Spec decision: may a pause send one stop or anchor per breathe light?

## zigbeat serve

- **PID 1 in a container.** The kernel ignores default-action signals for PID 1, so a second signal does not quit there. The phase 4 image needs an init (`docker run --init`, `init: true`).

## zigbeat setup and probe

- **The fixture column is a guess.** Setup's table shows the fixture each light would get if every offered light were chosen (`arrange` over all of them in `internal/setup/setup.go`). With only some chosen, a light alone in its stem gets a fixture named after itself instead.
