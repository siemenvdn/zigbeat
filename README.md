# zigbeat

**Make your Zigbee lights move with your music, without flooding your network.**

Think *groove2mqtt*: your music goes in, your existing Zigbee bulbs dance, and your motion sensors, switches and thermostats in the rest of the house keep working while the living room throws a party.

> **Status:** early. The network probe (`zigbeat probe`) works. The light engine itself is being built. See [Roadmap](#roadmap).

## Contents

1. [Why zigbeat](#why-zigbeat)
2. [What it does, and what it deliberately does not](#what-it-does-and-what-it-deliberately-does-not)
3. [Concepts: roles and fixtures](#concepts-roles-and-fixtures)
4. [How it works](#how-it-works)
5. [Effect design](#effect-design)
6. [Beat sources](#beat-sources)
7. [Configuration](#configuration)
8. [Getting started](#getting-started)
9. [Roadmap](#roadmap)
10. [Open questions](#open-questions)
11. [Design decisions](#design-decisions)
12. [Background: why Zigbee needs care](#background-why-zigbee-needs-care)
13. [Development](#development)
14. [References](#references)

## Why zigbeat

Music-synced lighting usually means a Hue Bridge with its Entertainment API, WLED strips, or extra hardware with a microphone. If your lights are ordinary Zigbee bulbs on [Zigbee2MQTT](https://www.zigbee2mqtt.io/), you are mostly out of luck:

- Spotify closed its audio analysis API in November 2024, so beats and tempo no longer come from Spotify.
- Zigbee is a slow, shared mesh. Blasting commands at a few bulbs fills the coordinator's queue, lights start lagging behind the music, and everything else in the house slows down with them.

zigbeat takes the opposite approach to a strobe: **fewer changes, on the beat, always on time.** It measures what your network can handle, stays well below that, and backs off the moment the network gets busy.

## What it does, and what it deliberately does not

**Does:**

- Changes colour lights every 2 to 5 seconds, always landing on a beat.
- Makes dimmable white lights breathe slowly with the music.
- Measures your network first and never exceeds the budget it found.
- Puts every light back exactly as it was when the show stops, also after a crash or Ctrl-C.
- Shows up in Home Assistant as a switch through MQTT discovery. No YAML in Home Assistant.

**Does not:**

| Not | Why |
|---|---|
| Flash on every beat | Zigbee cannot do that reliably. Hitting the beat matters more than speed. |
| Switch relays or smart plugs to the music | Mechanical relays wear out within a few evenings, click audibly and stress LED drivers with inrush current. |
| Use Zigbee groups | Group commands are broadcasts: every router in the house repeats them and they are the first thing to saturate. |
| Run through Home Assistant automations | Too much latency and jitter for timing. Home Assistant only turns the show on and off. |
| Need a Hue Bridge, WLED, a microphone or other hardware | It works with the bulbs you already have. |

## Concepts: roles and fixtures

zigbeat never needs to know what your lamps are called or where they hang. You describe them with two ideas:

**Roles** describe behaviour:

| Role | Needs | Does |
|---|---|---|
| `color` | Colour bulb (xy colour) | Changes colour on the beat, with a short brightness kick |
| `breathe` | Dimmable bulb | Brightness slowly rises and falls with the music |

Anything not in the configuration is never touched. That is also how relays and plugs stay safe: they simply have no role.

**Fixtures** group bulbs that physically belong together, like a pendant with three bulbs. Effects can then move per bulb or per fixture, which looks very different in a room.

## How it works

```
 beat source ──(beat: audible time, bpm, position in bar, energy)──▶ zigbeat engine ──MQTT──▶ Zigbee2MQTT ──▶ bulbs
                                                                         ▲   │
                                          on/off and status (MQTT discovery)  │ snapshot before, restore after
                                                                         │   ▼
                                                                  Home Assistant
```

The engine is a single Go binary, meant to run as a container next to Zigbee2MQTT.

| Part | Job |
|---|---|
| `BeatSource` | Interface that delivers beats with the moment they become audible. Sources are pluggable; the first one is a fixed-BPM metronome for development. |
| Planner | Decides per "moment" (every 2 to 5 s, on a beat) which bulbs change: colour, brightness kick, breathing direction. |
| Scheduler | Sends each command early by the network latency measured by `zigbeat probe`, plus a configurable audio offset for TV and amplifier delay, so the light lands on the beat. |
| Rate limiter | Token bucket set to the measured budget. A hard ceiling, never a target. |
| Closed loop | At most one command in flight per bulb and two or three in total. The next command for a bulb only goes out after Zigbee2MQTT confirmed the previous one. The coordinator queue never fills up, so other traffic always gets through. |
| Backoff | When confirmations slow down, the rate drops automatically. On Zigbee2MQTT errors the engine stops. The party gets calmer, the rest of the house keeps working. |
| Snapshot | Reads the state of every configured bulb at start and restores it at stop. |
| Home Assistant | MQTT discovery: a switch to start and stop, and a few sensors (status, current rate, latency). |

The MQTT client (`internal/mqtt`) and the Zigbee2MQTT layer (`internal/z2m`) exist and are tested.

## Effect design

### Cadence

A moment always falls on a beat, preferably at the start of a bar:

| Tempo | One bar takes | Moment |
|---|---|---|
| 60 to 90 BPM | 2.7 to 4 s | Every bar |
| 90 to 140 BPM | 1.7 to 2.7 s | Every bar, or every 2 bars below 2 s |
| above 140 BPM | below 1.7 s | Every 2 bars |

That keeps changes between 2 and 5 seconds apart, always on time.

### `color` bulbs

- **Chase:** at each moment, only some bulbs change colour, so colour travels through the room. Per bulb or per fixture.
- **Kick:** brightness jumps briefly on the moment, while the colour itself blends over 0.5 to 1 s.
- **Palette per track:** a set of colours that belong together. Later possibly derived from the album art.

### `breathe` bulbs

- Brightness moves with Zigbee's `brightness_move`: one message starts a smooth fade that the bulb performs itself.
- Each moment flips the direction: one message per bulb.
- Bulbs in the same fixture group can breathe in counter-phase.
- A limited range (for example 30% to 80%) keeps it atmospheric, not a strobe.

### Budget

Commands per second for a typical room:

| Cadence | 6 `color` bulbs | 2 `breathe` bulbs | Total |
|---|---|---|---|
| every 2 s | 3 cmd/s | 1 cmd/s | 4 cmd/s |
| every 5 s | 1.2 cmd/s | 0.4 cmd/s | 1.6 cmd/s |

zigbeat uses half of the highest clean rate `zigbeat probe` measured. For this room the network should therefore handle at least **8 cmd/s cleanly**. With less, zigbeat slows the cadence or changes fewer bulbs per moment.

## Beat sources

This is the biggest open piece. Candidates:

**A. Music Assistant (2.9 or later).** Music Assistant runs beat analysis on tracks and streams visualizer data over its Sendspin protocol. zigbeat would join as a Sendspin visualizer client.
- Music has to play through Music Assistant, for example via its Spotify Connect plugin.
- In an earlier experiment, visualizer players could not attach to a Chromecast as the playing device. Whether that is fixed is unknown.

**B. Tempo database plus playback position.** An external API (for example ReccoBeats, by Spotify track ID) provides the tempo; the Spotify integration in Home Assistant provides the position in the track. Together they give a beat grid. The phase of the first beat is uncertain and the API's reliability varies, but at a 2 to 5 second cadence a small phase error is much less visible.

**Ruled out:** microphones and other extra hardware; Spotify's own analysis API (closed).

Until a source is chosen, the engine is built against the `BeatSource` interface with a metronome.

## Configuration

*Planned for the engine; `zigbeat probe` currently takes flags.* One YAML file:

```yaml
mqtt:
  broker: mosquitto:1883
  username: zigbeat
  password_env: ZIGBEAT_MQTT_PASSWORD
  base_topic: zigbee2mqtt

budget:
  max_rate: 4          # cmd/s, written by `zigbeat probe`
  latency_ms: 105      # p50 at that load, used to send early
  audio_offset_ms: 0   # extra delay of TV and speakers, calibrate by ear

canary: hallway-ceiling

fixtures:
  - name: pendant
    role: color
    lights: [pendant-1, pendant-2, pendant-3]
  - name: floor-lamp
    role: breathe
    lights: [floor-lamp]
  - name: reading-lamp
    role: breathe
    lights: [reading-lamp]

effects:
  cadence: [2s, 5s]
  breathe_range: [30, 80]   # percent
```

Light names are Zigbee2MQTT friendly names. A planned `zigbeat setup` reads `zigbee2mqtt/bridge/devices`, lists your lights and suggests a role from what each device supports: colour becomes `color`, dimmable becomes `breathe`, on/off-only devices are not offered at all.

## Getting started

Today you can measure your network:

```bash
zigbeat probe -targets pendant-1,pendant-2,pendant-3 -canary hallway-ceiling
```

Pick three colour bulbs as targets and, as canary, a mains-powered device as far away from them as possible. The probe needs 5 to 10 minutes, your target bulbs will flash in all colours, and they are restored afterwards. Full guide: [`docs/probe.md`](docs/probe.md).

## Roadmap

### Phase 0: network probe ✅ built

`zigbeat probe` sends colour commands to the targets in rising steps until it finds the limit, while watching a canary device for the rest of the network. It stops at the first sign of overload and reports the clean rate, a safe budget and the latency to compensate.

### Phase 1: engine with a metronome

- Configuration (YAML), `BeatSource` interface, metronome, planner, scheduler, rate limiter, closed loop, backoff, snapshot and restore.
- Tested against `cmd/fake-z2m` first, then on real bulbs.
- Home Assistant integration through MQTT discovery.
- One-hour soak test with canary measurements alongside.

**Done when:** an hour without Zigbee2MQTT errors, the canary stays within its resting latency, and every bulb is restored at stop.

### Phase 2: a real beat source

- Implement source A or B (or both).
- Audio offset calibration.

**Done when:** colour changes land on the beat by ear for three tracks of different tempo.

### Phase 3: setup and effects

- `zigbeat setup`: discover devices, suggest roles, write the configuration.
- `zigbeat probe` writes its result into the configuration.
- Effect tuning: chase variants, palette per track, album art colours.

### Phase 4: release

- Container image, example `compose.yml`, documentation for Zigbee2MQTT and Home Assistant users.
- Releases for linux/amd64 and linux/arm64.

## Open questions

| Question | Matters for |
|---|---|
| Can a Sendspin visualizer listen along while a Chromecast plays? | Beat source A |
| How reliable is a tempo database for beat timing at a 2 to 5 s cadence? | Beat source B |
| Does Zigbee2MQTT confirm a command only after the bulb answered, or earlier? (See the sanity check in `docs/probe.md`) | Trusting the probe's latency |
| How much does the budget differ between coordinator types (EmberZNet, Z-Stack, deCONZ)? | Defaults for other users |
| Should ZHA (Home Assistant's own Zigbee stack) be supported later? | Reach |

## Design decisions

| Decision | Reason |
|---|---|
| Existing Zigbee bulbs only | The point of the project; no extra hardware |
| 2 to 5 s cadence on the beat, not every beat | On time beats fast, and it fits the Zigbee budget |
| Relays and plugs are never driven | Wear, clicking, inrush current |
| Unicast only, no groups | Broadcasts load the whole network and saturate first |
| Closed loop with a ceiling and backoff | The rest of the house always has priority |
| Home Assistant outside the timing path | Latency and jitter |
| Roles and fixtures instead of device names in code | Works in any home |
| Measure first, then build | The budget shapes the effect design |
| Go, standard library plus one YAML library (`go.yaml.in/yaml/v3`) | A single static binary; YAML is friendlier than JSON for people editing a config |

## Background: why Zigbee needs care

There is no single "commands per second" figure for a Zigbee network, but there are hard limits:

- **Broadcasts** (and therefore group commands) are limited by each router's broadcast transaction table. In EmberZNet a broadcast stays in that table for 3 seconds and each one is transmitted 3 times. Too many result in `NETWORK_BUSY`.
- **Unicasts** have a separate limit on how many can be in flight at once.
- **Zigbee2MQTT** has its own queue (`adapter_concurrent`, 1 to 64).
- **Bulbs** stop responding for a while after a burst of fast commands.
- Someone who fed LedFx straight into Zigbee2MQTT found about 6 updates per second workable; beyond that the queue grew and the lights reacted to old audio.

Bulb behaviour that zigbeat relies on (seen on IKEA bulbs, to be verified on others):

- A brightness transition is ignored by the firmware: brightness jumps immediately. Good for a crisp kick.
- Colour transitions do work, so a colour change can blend.
- A smooth brightness fade is possible with `brightness_move`.

## Development

```
cmd/
  zigbeat/      the binary: zigbeat probe, later run and setup
  fake-z2m/     simulated Zigbee2MQTT with a limited coordinator queue
internal/
  mqtt/         minimal MQTT 3.1.1 client (QoS 0, keepalive, wildcards)
  z2m/          Zigbee2MQTT topics: set, get, state, bridge/logging
  probe/        the network probe
docs/
  probe.md
```

```bash
go test ./...
go vet ./...

# build, e.g. for a Raspberry Pi
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o zigbeat ./cmd/zigbeat

# try the probe without real lights
mosquitto -p 18830 &
go run ./cmd/fake-z2m -broker 127.0.0.1:18830 &
go run ./cmd/zigbeat probe -broker 127.0.0.1:18830 -targets lamp-1,lamp-2,lamp-3 -canary far-lamp \
  -baseline 8s -step 6s -cooldown 2s
```

The fake handles about 10 commands per second by default: the probe should report OK up to around 8/s and stop at 10/s.

Contributor guidelines for humans and AI assistants: [`CLAUDE.md`](CLAUDE.md).

## References

- [Silicon Labs: Messaging (unicast, broadcast, multicast)](https://docs.silabs.com/zigbee/latest/zigbee-concepts-network/messaging)
- [Introduction of Zigbee Broadcast (Silicon Labs IoT Boot Camp)](https://github.com/Jim-tech/IoT-Developer-Boot-Camp/wiki/Introduction-of-Zigbee-Broadcast)
- [bellows PR #743: NETWORK_BUSY versus the unicast limit](https://github.com/zigpy/bellows/pull/743)
- [Zigbee2MQTT: adapter settings](https://www.zigbee2mqtt.io/guide/configuration/adapter-settings.html)
- [LedFx to Zigbee2MQTT, discussion #9621](https://github.com/Koenkk/zigbee2mqtt/discussions/9621)
- [Home Assistant issue #120754: bulbs unresponsive after fast commands](https://github.com/home-assistant/core/issues/120754)
- [Spotify's Audio Features deprecation and alternatives](https://www.musiciwant.com/studio/spotify-audio-features-alternative)
- [Music Assistant 2.9](https://www.music-assistant.io/blog/2026/06/10/music-assistant-2-9/)
- [Music Assistant: Hue Lights Sync plugin](https://www.music-assistant.io/plugins/hue-entertainment/)
- [Music Assistant PR #3901: WLED Audio Sync (closed)](https://github.com/music-assistant/server/pull/3901)
- [go.yaml.in/yaml, the maintained continuation of go-yaml](https://github.com/Go-yaml/yaml)
