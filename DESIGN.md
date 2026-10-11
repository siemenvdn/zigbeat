# zigbeat design

How zigbeat works and why. To use it, start with the [README](README.md).

## Contents

1. [Decisions](#decisions)
2. [Roles and fixtures](#roles-and-fixtures)
3. [Engine](#engine)
4. [Effects](#effects)
5. [Budget](#budget)
6. [Beat sources](#beat-sources)
7. [Configuration](#configuration)
8. [The probe](#the-probe)
9. [Roadmap](#roadmap)
10. [Open questions](#open-questions)
11. [Why Zigbee needs care](#why-zigbee-needs-care)
12. [Code layout](#code-layout)
13. [References](#references)

## Decisions

| Decision | Reason |
|---|---|
| Existing Zigbee bulbs only; no Hue Bridge, WLED, microphone or other hardware | The point of the project |
| One change every 2 to 5 s on the beat, never a flash on every beat | Zigbee cannot keep up with every beat reliably. Hitting the beat matters more than speed |
| Relays and smart plugs are never driven | Mechanical relays wear out within a few evenings, click audibly and stress LED drivers with inrush current |
| Unicast only, no Zigbee groups | Group commands are broadcasts: every router in the house repeats them and they are the first thing to saturate |
| Closed loop with a ceiling and backoff | The rest of the house always has priority |
| Home Assistant only switches the show on and off | Automations add too much latency and jitter for timing |
| Roles and fixtures instead of device names in code | Works in any home |
| Measure first, then build | The budget shapes the effect design |
| Go, standard library plus `go.yaml.in/yaml/v3` | A single static binary; YAML is friendlier than JSON to edit by hand |

## Roles and fixtures

zigbeat never needs to know what your lamps are called or where they hang. Two ideas describe them.

**Roles** describe behaviour:

| Role | Needs | Does |
|---|---|---|
| `color` | Colour bulb (xy colour) | Changes colour on the beat, with a short brightness kick |
| `breathe` | Dimmable bulb | Brightness slowly rises and falls with the music |

Anything without a role is never touched. That is also how relays and plugs stay safe.

**Fixtures** group bulbs that physically belong together, like a pendant with three bulbs. Effects can move per bulb or per fixture, which looks very different in a room.

## Engine

| Part | Job |
|---|---|
| `BeatSource` | Interface that delivers beats with the moment they become audible. Sources are pluggable; the first one is a fixed-BPM metronome for development. |
| Planner | Decides per moment (every 2 to 5 s, on a beat) which bulbs change: colour, brightness kick, breathing direction. |
| Scheduler | Sends each command early by the latency `zigbeat probe` measured, plus a configurable audio offset for TV and amplifier delay, so the light lands on the beat. |
| Rate limiter | Token bucket set to the measured budget. A hard ceiling, never a target. |
| Closed loop | At most one command in flight per bulb and two or three in total. The next command for a bulb only goes out after Zigbee2MQTT confirmed the previous one, so the coordinator queue never fills up. |
| Backoff | When confirmations slow down, the rate drops. On Zigbee2MQTT errors the engine stops. The party gets calmer, the rest of the house keeps working. |
| Snapshot | Reads the state of every configured bulb at start and restores it at stop. |
| Home Assistant | MQTT discovery: a switch to start and stop, and sensors for status, current rate and latency. |

## Effects

### Cadence

A moment always falls on a beat, preferably at the start of a bar:

| Tempo | One bar takes | Moment |
|---|---|---|
| 60 to 90 BPM | 2.7 to 4 s | Every bar |
| 90 to 140 BPM | 1.7 to 2.7 s | Every bar, or every 2 bars below 2 s |
| above 140 BPM | below 1.7 s | Every 2 bars |

That keeps changes between 2 and 5 seconds apart.

### `color` bulbs

- **Chase:** at each moment only some bulbs change colour, so colour travels through the room. Per bulb or per fixture.
- **Kick:** brightness jumps briefly on the moment, while the colour blends over 0.5 to 1 s.
- **Palette per track:** a set of colours that belong together, later possibly derived from the album art.

### `breathe` bulbs

- Brightness moves with Zigbee's `brightness_move`: one message starts a smooth fade that the bulb performs itself.
- Each moment flips the direction: one message per bulb.
- Bulbs in the same fixture can breathe in counter-phase.
- A limited range (for example 30% to 80%) keeps it atmospheric.

## Budget

Commands per second for a typical room:

| Cadence | 6 `color` bulbs | 2 `breathe` bulbs | Total |
|---|---|---|---|
| every 2 s | 3 cmd/s | 1 cmd/s | 4 cmd/s |
| every 5 s | 1.2 cmd/s | 0.4 cmd/s | 1.6 cmd/s |

zigbeat uses half of the highest clean rate the probe measured, so this room needs a network that handles at least 8 cmd/s cleanly. With less, zigbeat slows the cadence or changes fewer bulbs per moment.

## Beat sources

The biggest open piece. Candidates:

**A. Music Assistant (2.9 or later).** Music Assistant runs beat analysis on tracks and streams visualizer data over its Sendspin protocol. zigbeat would join as a Sendspin visualizer client.
- Music has to play through Music Assistant, for example via its Spotify Connect plugin.
- In an earlier experiment, visualizer players could not attach to a Chromecast as the playing device. Whether that is fixed is unknown.

**B. Tempo database plus playback position.** An external API (for example ReccoBeats, by Spotify track ID) provides the tempo; the Spotify integration in Home Assistant provides the position in the track. Together they give a beat grid. The phase of the first beat is uncertain and the API's reliability varies, but at a 2 to 5 s cadence a small phase error is much less visible.

**Ruled out:** microphones and other extra hardware; Spotify's audio analysis API (closed in November 2024).

Until a source is chosen, the engine is built against the `BeatSource` interface with a metronome.

## Configuration

Planned for the engine. One YAML file:

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

Light names are Zigbee2MQTT friendly names. A planned `zigbeat setup` reads `zigbee2mqtt/bridge/devices`, lists the lights and suggests a role from what each device supports: colour becomes `color`, dimmable becomes `breathe`, on/off-only devices are not offered.

## The probe

`zigbeat probe` sends colour commands to the targets in steps of 1, 2, 3, 4, 6, 8, 10 and 12 commands per second (all targets together), then climbs by 25% per step (15, 19, 24, 30, ...) up to at most 100. Each step takes 20 s with 10 s of rest in between. For every command it measures the time until Zigbee2MQTT confirms the new state. Meanwhile it asks the canary for its state about once per second.

Safety rails:

- **Baseline first:** 20 s with only the canary, to know what normal looks like.
- **Self-check** before the first step: every target must return a recognisable confirmation, or the probe stops.
- **Stops at the first overload.** A step fails when more than 5% of commands go unanswered, latency exceeds 1.5 s or keeps rising, the canary slows down clearly (3 times its resting latency or 300 ms more) or Zigbee2MQTT logs errors. The probe then waits for the network to settle and goes no higher.
- **Warnings are measured twice.** A step with a warning (one lost command, one Zigbee2MQTT error, ...) is repeated at the same rate. A second warning ends the run.
- **One command in flight per bulb.** When all targets are still busy too often, that ends the climb too; more targets may go higher.
- **Restore** of every target at the end, also after Ctrl-C.
- **Unicast only.**

## Roadmap

### Phase 0: network probe (done)

`zigbeat probe` finds the network limit while watching a canary, and reports the clean rate, a safe budget and the latency to compensate.

### Phase 1: engine with a metronome

- Configuration, `BeatSource` interface, metronome, planner, scheduler, rate limiter, closed loop, backoff, snapshot and restore.
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
| Does Zigbee2MQTT confirm a command only after the bulb answered, or earlier? | Trusting the probe's latency |
| How much does the budget differ between coordinator types (EmberZNet, Z-Stack, deCONZ)? | Defaults for other users |
| Should ZHA (Home Assistant's own Zigbee stack) be supported later? | Reach |

## Why Zigbee needs care

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

## Code layout

```
cmd/
  zigbeat/      the binary: zigbeat probe, later run and setup
  fake-z2m/     simulated Zigbee2MQTT with a limited coordinator queue
internal/
  mqtt/         minimal MQTT 3.1.1 client (QoS 0, keepalive, wildcards)
  z2m/          Zigbee2MQTT topics: set, get, state, bridge/logging
  probe/        the network probe
```

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
