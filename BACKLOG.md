# Backlog

Small known issues, found in the reviews of phases 1a to 2 and left for later. Plans and decisions live in [DESIGN.md](DESIGN.md).

## zigbeat run

- **A silent broker drop can still pause the show.** A drop that only the keepalive finds (up to 30 s) gives the lights losses that raise backoff. After the reconnect their failure counts start over and lights left alone are read at once, but a pause holds those reads until the backoff steps down, up to a minute. Idea: a loss while the canary does not answer either is the network's, not the light's.
- **A pause freezes lights mid-effect.** During a backoff pause, a light left alone or an outage, breathe lights keep moving until 100% or their minimum, and colour lights can stay at the kick level because the queued return is cleared. Breathe lights get their low end when the pause ends (`Planner.Settle`); during it they still run free. Spec decision: may a pause send one stop or anchor per breathe light?

## zigbeat serve

- **PID 1 in a container.** The kernel ignores default-action signals for PID 1, so a second signal does not quit there. The phase 4 image needs an init (`docker run --init`, `init: true`).

## zigbeat setup and probe

- **The fixture column is a guess.** Setup's table shows the fixture each light would get if every offered light were chosen (`arrange` over all of them in `internal/setup/setup.go`). With only some chosen, a light alone in its stem gets a fixture named after itself instead.
