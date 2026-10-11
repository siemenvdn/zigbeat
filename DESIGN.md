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
| Network trouble pauses the show, never ends it | A party should not stop because one bulb or the broker hiccups. A pause is the deepest backoff, so the network still always wins |
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
| `BeatSource` | Interface that delivers beats at least 1 s before they become audible. Sources are pluggable; the first one is a fixed-BPM metronome. |
| Planner | Decides per moment which bulbs change: colour, brightness kick, breathing direction. See [Effects](#effects). |
| Scheduler | Sends each message early by its number of Zigbee frames times the measured latency per frame, plus a configurable audio offset for TV and amplifier delay. The estimate starts at `budget.latency_ms` and follows the median of the last 20 confirmations. A moment that cannot leave within 150 ms of its send time is dropped as a whole; returns and breathe are never dropped. Due messages for one bulb are merged. |
| Rate limiter | Token bucket in Zigbee frames at `budget.max_rate`, holding one moment (6 frames). A hard ceiling, never a target. Colour plus brightness in one message is two frames. |
| Closed loop | At most one message in flight per bulb and three in total. A message is confirmed by the bulb's next state; a `brightness_move` publishes none, so it frees its slot after 1 s. Unanswered after 5 s counts as lost. |
| Backoff | Level 0 normal, 1 no kick and one message in flight, 2 half as many moments, 3 pause: nothing is sent but canary reads. Signals: latency per frame above twice the show's own starting median, a lost message, a slow canary, a Zigbee2MQTT error. One step per 10 s at most; one step back after a quiet minute. Three errors about our bulbs within a minute pause at once. Errors about other devices raise it to 1 at most. |
| Robustness | A bulb that misses twice in a row is left alone and read once a minute until it answers. A lost broker connection is dialled again every 5 s. Bulbs that do not answer at the start are left out, and a Zigbee2MQTT group name in the config is refused. A panic restarts the engine; the third one stops the show. |
| Snapshot | Reads the state of every configured bulb at start and restores it at stop, one bulb at a time within the budget. A bulb that was off first gets its level and colour back, then goes off, so it does not come on in the show's colour next time. A broker that drops during the restore is dialled again for 30 s. |
| Home Assistant | MQTT discovery: a switch to start and stop, and sensors for status, current rate and latency. |

The latency signal is relative to the show itself, not to the probe: a moment sends up to three messages at once, they queue at the coordinator, and the show settles at about twice the probe's evenly spaced latency.

## Effects

### Cadence

A moment falls every 1, 2, 4, 8 or 16 beats: the shortest that lasts at least `effects.cadence` (2 s by default). From 4 beats on that is the first beat of a bar:

| Tempo | Moment |
|---|---|
| 30 to 60 BPM | Every 2 beats |
| 61 to 120 BPM | Every bar |
| 121 to 240 BPM | Every 2 bars |
| 241 to 300 BPM | Every 4 bars |

Backoff level 2 doubles the interval.

### `color` bulbs

- **Chase:** at each moment the next bulbs in config order change, about half of them and at most three messages, so colour travels through the room.
- **Fixture mode:** `separate` changes one bulb at a time, `together` the whole fixture at once in one colour, `alternate` switches between the two every 4 moments (the default).
- **Colour:** the next palette colour that no bulb of the same fixture shows. Zigbee2MQTT gets it as hex and converts it per bulb.
- **Kick:** on the moment, colour (blending over 0.7 s) and brightness 30 points above the base in one message; one beat later back to the base. From backoff level 1 the kick is left out.
- **Start:** every bulb is switched on at the base brightness with a first colour.
- **Later:** a palette per track, possibly from the album art.

### `breathe` bulbs

- Brightness moves with Zigbee's `brightness_move`: one message starts a smooth fade that the bulb performs itself, at a rate that reaches the other end of `breathe_range` exactly at the next moment.
- Each moment flips the direction: one message per bulb.
- Every 4th moment also sets the low end as an absolute brightness, against drift.
- Later: bulbs in the same fixture breathing in counter-phase.

## Budget

Zigbee frames per second for a typical room. A colour change with its kick and return is 3 frames; three bulbs change per moment. A breathe bulb takes 1 frame per moment, 2 every 4th:

| Moment every | 6 `color` bulbs | 2 `breathe` bulbs | Total |
|---|---|---|---|
| 2 s | 4.5 frames/s | 1.25 frames/s | about 6 frames/s |
| 4 s | 2.25 frames/s | 0.6 frames/s | about 3 frames/s |

The budget is half of the highest clean rate the probe measured, so this room wants a network that handles about 12 frames/s cleanly. With less, moments are dropped, and backoff first leaves out the kick (1 frame per change instead of 3), then halves the moments.

## Beat sources

The biggest open piece. Candidates:

**A. Music Assistant (2.9 or later).** Music Assistant runs beat analysis on tracks and streams visualizer data over its Sendspin protocol. zigbeat would join as a Sendspin visualizer client.
- Music has to play through Music Assistant, for example via its Spotify Connect plugin.
- In an earlier experiment, visualizer players could not attach to a Chromecast as the playing device. Whether that is fixed is unknown.

**B. Tempo database plus playback position.** An external API (for example ReccoBeats, by Spotify track ID) provides the tempo; the Spotify integration in Home Assistant provides the position in the track. Together they give a beat grid. The phase of the first beat is uncertain and the API's reliability varies, but at a 2 to 5 s cadence a small phase error is much less visible.

**Ruled out:** microphones and other extra hardware; Spotify's audio analysis API (closed in November 2024).

Until a source is chosen, the engine is built against the `BeatSource` interface with a metronome.

## Configuration

One YAML file; [`config.example.yaml`](config.example.yaml) is a working example:

```yaml
mqtt:
  broker: mosquitto:1883             # default 127.0.0.1:1883
  username: zigbeat
  password_env: ZIGBEAT_MQTT_PASSWORD
  base_topic: zigbee2mqtt            # default

budget:
  max_rate: 15          # Zigbee frames per second, from `zigbeat probe`, required
  latency_ms: 66        # p50 at that load, from `zigbeat probe`, required
  audio_offset_ms: 0    # extra delay of TV and speakers, calibrate by ear

canary: hallway-ceiling # optional

fixtures:
  - name: pendant
    role: color
    mode: alternate     # alternate (default), separate or together
    lights: [pendant-1, pendant-2, pendant-3]
  - name: floor-lamp
    role: breathe
    lights: [floor-lamp]

effects:
  cadence: 2s                 # minimum time between moments, default 2s
  brightness: 60              # percent, base of color bulbs, default 60
  palette: ["#ff2a6d", "#ff7b00", "#ffd400", "#00e5ff", "#2962ff", "#aa00ff"]  # the default
  breathe_range: [30, 80]     # percent, default [30, 80]
```

Unknown keys are an error, so typos do not pass silently. Every light belongs to exactly one fixture, and the canary is not one of the lights.

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

Small known issues that are not planned yet: [BACKLOG.md](BACKLOG.md).

### Phase 0: network probe (done)

`zigbeat probe` finds the network limit while watching a canary, and reports the clean rate, a safe budget and the latency to compensate.

### Phase 1: engine with a metronome

- **1a (done):** `zigbeat run`: configuration, `BeatSource` interface, metronome, planner, scheduler, rate limiter, closed loop, backoff with pause, robustness, snapshot and restore. Tested against `cmd/fake-z2m`.
- **1b:** Home Assistant integration through MQTT discovery.
- **1c:** one-hour soak test on real bulbs with canary measurements alongside.

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
| Does the colour still blend when colour and brightness arrive in one message (IKEA)? | The kick |
| Is the 50 to 70 ms ripple across a `together` fixture visible? | Fixture mode |
| Does an absolute brightness end a running `brightness_move` on every bulb? | Restore after breathe |
| Does breathe stay within its range over an hour? | Re-anchoring every 4th moment |
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
  zigbeat/        the binary: zigbeat probe and run, later setup
  fake-z2m/       simulated Zigbee2MQTT: one coordinator queue, time per frame, optional loss, groups
internal/
  cli/            what the subcommands share: stop signals, flag errors, the time format
  mqtt/           minimal MQTT 3.1.1 client (QoS 0, retained publish, keepalive, wildcards)
  z2m/            Zigbee2MQTT topics, groups, frame count, the canary limit
  lights/         snapshot and restore of lights, shared by probe and run
  config/         the YAML configuration
  beat/           beats, the Source interface, the metronome
  run/            zigbeat run: connection, startup, the real-time loop, stop and restore
    show/         the planner: moments, chase, colours, kick, breathe
    flow/         frame budget, one message in flight per light, backoff, canary
    engine/       the show as a state machine without I/O
  probe/          zigbeat probe: session, rate steps, climb
    measure/      samples, statistics, verdicts, command tracking, the canary
    palette/      commands whose echoes can be told apart
    report/       the step table, the summary and the CSV
```

Packages are small and split by concern; a file rarely passes 150 lines.

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
