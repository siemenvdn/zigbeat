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
| An allowlist decides what may be driven: `zigbeat setup` offers a device only when every rule holds, and run, serve and the probe check every light against the same rules | Some relay inserts, dimming plugs and dimmer modules present themselves as a light with brightness. A bulb refused wrongly is an inconvenience; a relay driven is what must never happen |
| `zigbeat setup` asks two questions and writes a file to edit; on an existing file it replaces only the lights and the canary | Picking a few lights from a list is quick; the budget, the offset and the effects stay |
| The configuration is edited in place: only the values that change move | Comments, blank lines and alignment belong to the user; no YAML library keeps them all |
| Unicast only, no Zigbee groups | Group commands are broadcasts: every router in the house repeats them and they are the first thing to saturate |
| Closed loop with a ceiling and backoff | The rest of the house always has priority |
| Network trouble pauses the show, never ends it | A party should not stop because one bulb or the broker hiccups. A pause is the deepest backoff, so the network still always wins |
| Home Assistant only switches the show on and off | Automations add too much latency and jitter for timing. The tempo comes from the beat source, not from a slider |
| `zigbeat serve` is a separate command; `zigbeat run` stays the one-shot CLI | The service that Home Assistant switches has a different life: it waits, plays, restores and waits again |
| serve has its own MQTT connection; each show opens its own | A stuck Home Assistant link never slows the show, and an idle serve subscribes to no light |
| serve always starts with the show off | A restart must never start a show on its own |
| The beats come from Music Assistant's own analysis, through its visualizer relay | Real beat times with downbeats, on any speaker Music Assistant plays to and from any of its providers. See [Beat sources](#beat-sources) |
| Own WebSocket client, no dependency | The relay needs no extensions, compression or TLS; about 200 lines, like the MQTT client |
| The audio offset is calibrated by ear while `zigbeat run` plays | One number per speaker setup; Home Assistant stays on and off only |
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

`zigbeat setup` proposes the role from Zigbee2MQTT's device list: a light with xy colour becomes `color`, any other dimmable bulb `breathe`. It offers only enabled, interviewed routers with a native definition and exactly one light on one channel, with settable state and brightness, without switch, cover, fan, lock or climate parts, power metering, in-wall settings or a description that reads like a plug, relay, dimmer or module; Tuya datapoint lights are left out too. `zigbeat run`, `serve` and `probe` check every light against the same rules before anything changes.

**Fixtures** group bulbs that physically belong together, like a pendant with three bulbs. Effects can move per bulb or per fixture, which looks very different in a room.

## Engine

| Part | Job |
|---|---|
| `BeatSource` | Interface that delivers beats at least 1 s before they become audible. Music Assistant is the source when the config has it, else a fixed-BPM metronome. See [Beat sources](#beat-sources). |
| Planner | Decides per moment which bulbs change: colour, brightness kick, breathing direction. See [Effects](#effects). |
| Scheduler | Sends each message early by its number of Zigbee frames times the measured latency per frame, plus a configurable audio offset for TV and amplifier delay, which `zigbeat run` can change while it plays. The estimate starts at `budget.latency_ms` and follows the median of the last 20 confirmations. A moment that cannot leave within 150 ms of its send time is dropped as a whole; returns and breathe are never dropped. Due messages for one bulb are merged. |
| Rate limiter | Token bucket in Zigbee frames at `budget.max_rate`, holding one moment (6 frames). A hard ceiling, never a target. Colour plus brightness in one message is two frames. |
| Closed loop | At most one message in flight per bulb and three in total. A message is confirmed by the bulb's next state; a `brightness_move` publishes none, so it frees its slot after 1 s. Unanswered after 5 s counts as lost. |
| Backoff | Level 0 normal, 1 no kick and one message in flight, 2 half as many moments, 3 pause: nothing is sent but canary reads. Signals: latency per frame above twice the show's own starting median, a lost message, a slow canary, a Zigbee2MQTT error. One step per 10 s at most; one step back after a quiet minute. Three errors about our bulbs within a minute pause at once. Errors about other devices raise it to 1 at most. |
| Robustness | A bulb that misses twice in a row is left alone and read once a minute until it answers. A lost broker connection is dialled again every 5 s. Bulbs that do not answer at the start are left out, and a Zigbee2MQTT group name in the config is refused. A panic restarts the engine; the third one stops the show. |
| Snapshot | Reads the state of every configured bulb at start and restores it at stop, one bulb at a time within the budget. A bulb that was off first gets its level and colour back, then goes off, so it does not come on in the show's colour next time. A broker that drops during the restore is dialled again for 30 s. |
| Home Assistant | `zigbeat serve`: MQTT device discovery with a switch and sensors, availability through a will. See [Home Assistant](#home-assistant). |

The latency signal is relative to the show itself, not to the probe: a moment sends up to three messages at once, they queue at the coordinator, and the show settles at about twice the probe's evenly spaced latency.

### Home Assistant

| Topic | Retained | Payload |
|---|---|---|
| `homeassistant/device/zigbeat/config` | yes | Device discovery: one switch and six sensors in one message |
| `zigbeat/availability` | yes | `online` after connecting; `offline` on a clean stop and as the MQTT will |
| `zigbeat/state` | yes | One JSON object for every entity; the numbers are `null` (unknown) while the show does not play |
| `zigbeat/show/set` | no | `ON` or `OFF` from the switch |

- On every connect serve clears a retained command first, subscribes, then publishes `online`, the discovery message and the last state. A broker restart drops the connection, so this also covers a broker that lost its retained messages; Home Assistant's birth message is not needed.
- Each show calls back every 10 s with its numbers. The callback only stores the newest state; a separate goroutine publishes it, so Home Assistant stays out of the timing path.
- `OFF` cancels the show, which restores every light exactly as on Ctrl-C. An `ON` that arrives during the restore starts the next show right after it.

## Effects

### Cadence

A moment falls every 1, 2, 4, 8 or 16 beats: the shortest that lasts at least `effects.cadence` (2 s by default). From 4 beats on that is the first beat of a bar:

| Tempo | Moment |
|---|---|
| 30 BPM | Every beat |
| 31 to 60 BPM | Every 2 beats |
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
- When the beats stop (a pause, a new track, a first play waiting for its analysis), each bulb gets its low end once, a beat after the last beat; otherwise its fade would run on to the bulb's own limit.
- Later: bulbs in the same fixture breathing in counter-phase.

## Budget

Zigbee frames per second for a typical room. A colour change with its kick and return is 3 frames; three bulbs change per moment. A breathe bulb takes 1 frame per moment, 2 every 4th:

| Moment every | 6 `color` bulbs | 2 `breathe` bulbs | Total |
|---|---|---|---|
| 2 s | 4.5 frames/s | 1.25 frames/s | about 6 frames/s |
| 4 s | 2.25 frames/s | 0.6 frames/s | about 3 frames/s |

The budget is half of the highest clean rate the probe measured, so this room wants a network that handles about 12 frames/s cleanly. With less, moments are dropped, and backoff first leaves out the kick (1 frame per change instead of 3), then halves the moments.

## Beat sources

The source is Music Assistant (MA) 2.10 or later. Its Smart Fades analysis runs Beat This!, a neural beat tracker with downbeats, on the audio MA decodes for playback, and its MilkDrop Visualizer plugin relays the beats over a WebSocket on MA's webserver. That works on any speaker MA plays to, a Chromecast included, and for every MA provider, but not for audio MA only passes on: Spotify Connect into MA and radio have no beats.

| Step | How |
|---|---|
| Connect | `ws://<ma>:8095/milkdrop_visualizer`, optionally `?player=<id>`; a long-lived MA token as the first message. Without a player, MA picks the one that is playing when zigbeat connects, and zigbeat connects again when the music stops, so it follows whichever player plays next; after a pause the beats return within 5 s of the resume |
| Clock | Bursts of 8 `client/time` requests every 10 s; the answer with the shortest round trip in the last 30 s maps MA's monotonic clock onto zigbeat's |
| Beats | Binary frames `[17][timestamp µs][downbeat]`, every remaining beat of a track at once when the analysis is there: on a first play about 5 to 10 s into the track, at once when it is cached. `stream/clear` (new track, seek, resume) and `stream/end` drop them, and the source tells the show, which drops the moments it planned from them that have not left |
| Numbering | `Index` counts per track and jumps to the next multiple of 4 at every downbeat, so moments stay on the first beat of a bar. The tempo is the median interval of the track |
| Delivery | Each beat 1 s before it sounds, once the burst of a track is complete (200 ms without a new beat), so the first beat is numbered from its downbeat; a beat less than 0.9 s ahead by then is skipped |
| Trouble | Notes on change only; a lost connection or no player is retried every 5 s, a refused token every 60 s. The source never ends the show |

The relay anchors a track's beats on the position the player reports. A constant error, such as TV and amplifier delay, is the audio offset; an error that differs per track is not (an open question).

**Rejected:**

- **Sendspin visualizer.** It only gets data inside a Sendspin group; a Chromecast joins only through MA's experimental Sendspin over Cast, which is rarely in sync without a delay set by hand per speaker. Sendspin 1.0 also requires Noise encryption and pairing. A later upgrade behind the same `BeatSource`.
- **Tempo database plus playback position.** No free database gives the time of the first beat; Home Assistant's Spotify integration reports the position in whole seconds every 30 s; databases disagree by 0.1 to 0.2 BPM. A change every 2 to 5 s is judged like a single flash and beep (noticed from about 45 ms late or 125 ms early, ITU-R BT.1359), so a random phase misses most of the time and a rounded tempo drifts off within half a minute.
- Microphones and other extra hardware; Spotify's audio analysis API (closed in November 2024).

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
  audio_offset_ms: 0    # extra delay of TV and speakers, -500 to 2000, calibrate in zigbeat run

music_assistant:        # optional; without it the show runs to a metronome
  url: ws://music-assistant:8095
  token_env: ZIGBEAT_MA_TOKEN
  player: ""            # optional MA player id; empty follows whichever player is playing

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

Light names are Zigbee2MQTT friendly names. `zigbeat setup` writes the file with a budget of zeros; `zigbeat probe -config` fills in `max_rate` and `latency_ms`, with the date and the measured rate in the comment, and typing `save` in `zigbeat run` writes `audio_offset_ms`. Each of them changes only those values: comments, blank lines and alignment stay byte for byte. A value they cannot change safely, such as a quoted one, is left alone with a message naming the value to set by hand.

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

Small known issues that are not planned yet: [BACKLOG.md](BACKLOG.md). Ideas for later: [IDEAS.md](IDEAS.md).

### Phase 0: network probe (done)

`zigbeat probe` finds the network limit while watching a canary, and reports the clean rate, a safe budget and the latency to compensate.

### Phase 1: engine with a metronome

- **1a (done):** `zigbeat run`: configuration, `BeatSource` interface, metronome, planner, scheduler, rate limiter, closed loop, backoff with pause, robustness, snapshot and restore. Tested against `cmd/fake-z2m`.
- **1b (done):** `zigbeat serve`: a Home Assistant switch and sensors through MQTT discovery. Tested against `cmd/fake-z2m` and a throwaway Home Assistant container.
- **1c:** one-hour soak test on real bulbs with canary measurements alongside.

**Done when:** an hour without Zigbee2MQTT errors, the canary stays within its resting latency, and every bulb is restored at stop.

### Phase 2: a real beat source

- **Built:** Music Assistant as the beat source, live audio offset calibration, breathe settling in gaps, `zigbeat beats`. Tested against `cmd/fake-ma` and `cmd/fake-z2m`.
- **To do:** the check on real speakers and lights.

**Done when:** colour changes land on the beat by ear for three tracks of different tempo.

### Phase 3: setup and effects

- **3a (built):** `zigbeat setup` with an allowlist of what may be driven, `zigbeat probe -config` writing the budget, `save` for the audio offset, and the check of every light at the start. Tested against `cmd/fake-z2m`. To do: classify the device list of a real home.
- **3b:** effect tuning: chase variants, a palette per track from the album art, energy from the music.

### Phase 4: release

- Container image, example `compose.yml` with a stop grace period long enough for a restore (Docker waits only 10 s by default), documentation for Zigbee2MQTT and Home Assistant users.
- Releases for linux/amd64 and linux/arm64.

## Open questions

| Question | Matters for |
|---|---|
| How well does Music Assistant's Cast player report its position at a track start, and does the error differ per track? | One audio offset for every track |
| How long does Smart Fades take on a Raspberry Pi 5 before the beats of a first play arrive? | The quiet start of new tracks |
| Do downbeats arrive on the user's Music Assistant? | Moments on the first beat of a bar |
| Does Music Assistant's experimental visualizer relay stay as it is in later releases? | The beat source |
| Does Zigbee2MQTT confirm a command only after the bulb answered, or earlier? | Trusting the probe's latency |
| Does the colour still blend when colour and brightness arrive in one message (IKEA)? | The kick |
| Is the 50 to 70 ms ripple across a `together` fixture visible? | Fixture mode |
| Does an absolute brightness end a running `brightness_move` on every bulb? | Restore after breathe |
| Does breathe stay within its range over an hour? | Re-anchoring every 4th moment |
| How much does the budget differ between coordinator types (EmberZNet, Z-Stack, deCONZ)? | Defaults for other users |
| Should ZHA (Home Assistant's own Zigbee stack) be supported later? | Reach |
| Does the allowlist offer every bulb in a real home, and nothing else? | What setup offers, and the start check |
| Do bulbs with only `color_hs` show the colours zigbeat sends as hex? | Offering them as `color` |

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
  zigbeat/        the binary: zigbeat setup, probe, run, serve and beats
  fake-z2m/       simulated Zigbee2MQTT: one coordinator queue, time per frame, optional loss, groups, kinds of device, a landing log
  fake-ma/        simulated Music Assistant relay: tracks, analysis delay, seeks, gaps, its own clock
internal/
  cli/            what the subcommands share: stop signals, flag errors, the time stamp, the 5 s redial
  mqtt/           minimal MQTT 3.1.1 client (QoS 0, retained publish and will, live-only subscribe, keepalive, wildcards)
  ws/             minimal WebSocket client, and the server side for tests and fake-ma
  z2m/            Zigbee2MQTT topics, groups, the device list and which lights may be driven, frame count, the canary limit
  lights/         snapshot and restore of lights, shared by probe and run
  config/         the YAML configuration, and editing it in place
  beat/           beats, the Source interface, the metronome
    ma/           the Music Assistant source: connection, clock sync, beat grid
  beats/          zigbeat beats: the beats from Music Assistant, printed without touching a light
  setup/          zigbeat setup: the device survey, the questions, fixtures, writing the config
  run/            zigbeat run: connection, startup, the real-time loop, stop and restore
    show/         the planner: moments, chase, colours, kick, breathe
    flow/         frame budget, one message in flight per light, backoff, canary
    engine/       the show as a state machine without I/O
  serve/          zigbeat serve: the Home Assistant link, the switch and the show lifecycle
    ha/           Home Assistant topics, the discovery message and the state payload
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
- [Music Assistant: the MilkDrop visualizer relay (relay.py, tap.py)](https://github.com/music-assistant/server/tree/dev/music_assistant/providers/milkdrop_visualizer)
- [Music Assistant: Smart Fades, the beat analysis](https://github.com/music-assistant/server/tree/dev/music_assistant/providers/smart_fades)
- [Music Assistant PR #6081: Sendspin over Cast off by default](https://github.com/music-assistant/server/pull/6081)
- [Sendspin protocol](https://github.com/Sendspin/spec)
- [Home Assistant's Spotify integration: position in whole seconds, polled every 30 s](https://github.com/home-assistant/core/tree/dev/homeassistant/components/spotify)
- [ITU-R BT.1359: relative timing of sound and vision](https://www.itu.int/rec/R-REC-BT.1359)
- [RFC 6455: the WebSocket protocol](https://www.rfc-editor.org/rfc/rfc6455)
- [go.yaml.in/yaml, the maintained continuation of go-yaml](https://github.com/Go-yaml/yaml)
