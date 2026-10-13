# Working on zigbeat

Read the [README](README.md) and [DESIGN.md](DESIGN.md) first. The README is for users; DESIGN.md holds the architecture, roadmap and design decisions. This file covers how work is done.

Keep the README short and about what works today. Plans, rationale and research go in DESIGN.md.

## Language and style

- Everything in English: code, comments, docs, CLI output, config keys, commit messages.
- No comments: a name says what it does. The only exception is one short line on an exported name, for `go doc`. A number is a named constant; its reason lives in DESIGN.md, not next to it.
- Identifiers spell `color`, as the config keys, Zigbee2MQTT and go-colorful do; prose and output spell `colour`.
- Never put names of real lights, rooms, hosts or accounts from anyone's home in the repo. Examples use generic names such as `pendant-1`, `floor-lamp`, `hallway-ceiling`.

## Code

- Go. Use a well-maintained library when it makes our code smaller; propose each new dependency first, with the reason. In use: `go.yaml.in/yaml/v3` for configuration, `github.com/lucasb-eyer/go-colorful` for colour conversion, `github.com/eclipse/paho.mqtt.golang` for MQTT, `golang.org/x/time/rate` for the frame budget and `github.com/gorilla/websocket` for the Music Assistant relay.
- Before every commit: `gofmt -l .` is empty, `go vet ./...` and `go test ./...` pass.
- The binary is static: `CGO_ENABLED=0`. Release targets are linux/amd64 and linux/arm64.
- New Zigbee behaviour is tested against `cmd/fake-z2m` first. Extend the fake when it cannot simulate something.
- Subcommands live under `cmd/zigbeat`; their logic lives in `internal/<command>`.

## Commits

All lowercase, English, descriptive, no conventional-commit prefixes. Example: `add token bucket rate limiter`.

## Releases

- A release is an annotated tag `vX.Y.Z` on `main`, pushed by a human after the checks on real hardware. `.github/workflows/release.yml` then tests and publishes the binaries and the image.
- Never push a tag, an image or a GitHub release yourself. `goreleaser release --snapshot --clean` stays local.

## Zigbee: hard rules

- Never use Zigbee groups or broadcasts. Unicast only, to `<base_topic>/<friendly name>/set`.
- Never drive relays or plugs. Only devices with a role in the config are touched.
- The budget from `zigbeat probe` is a ceiling, not a target.
- At most one command in flight per bulb; the next one only after Zigbee2MQTT confirmed the previous one.
- On Zigbee2MQTT errors or rising latency: back off or stop, never push through.
- Always restore every bulb to its state from before the start, also on Ctrl-C, SIGTERM or an error.

## Real networks

- Never send commands to a real broker or real bulbs yourself. Runs on real hardware are done by a human; provide the binary, the exact command and what they will see.
- Never commit credentials. Passwords come from environment variables.

## Home Assistant

- zigbeat registers itself through MQTT discovery. Users should not need any Home Assistant YAML.
