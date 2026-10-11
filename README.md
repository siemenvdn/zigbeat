# zigbeat

**Make your Zigbee lights move with your music, without flooding your network.**

Ordinary Zigbee bulbs on [Zigbee2MQTT](https://www.zigbee2mqtt.io/) have no good way to follow music. Spotify closed its audio analysis API, and blasting commands at a few bulbs fills the shared mesh until the lights lag and the rest of the house slows down. zigbeat does the opposite of a strobe: fewer changes, always on the beat, within a budget measured on your own network.

## Status

- **Works today:** `zigbeat probe` measures how many light commands per second your network handles. `zigbeat run` drives your lights to a fixed-BPM metronome within that budget. `zigbeat serve` does the same as a service that Home Assistant switches on and off.
- **In progress:** a real beat source instead of the metronome.
- **Later:** `zigbeat setup`, a container image.

## What it does

- Colour bulbs change colour on the beat, a few at a time, with a short brightness kick.
- Dimmable bulbs breathe slowly up and down.
- It stays below the budget the probe found, backs off when the network gets busy and pauses when it keeps struggling. Network trouble never ends the show.
- Every light goes back to how it was when you stop the show, with Ctrl-C or the switch in Home Assistant.
- Home Assistant gets a switch and sensors through MQTT discovery, without any YAML.

It does not flash on every beat, use Zigbee groups, or switch relays and plugs. [DESIGN.md](DESIGN.md) explains why.

## How it works

```
 beat source ──(beat: audible time, bpm, position in bar, energy)──▶ zigbeat engine ──MQTT──▶ Zigbee2MQTT ──▶ bulbs
                                                                         ▲   │
                                          on/off and status (MQTT discovery)  │ snapshot before, restore after
                                                                         │   ▼
                                                                  Home Assistant
```

A beat source says when each beat becomes audible. The engine picks a few bulbs per moment and sends each command early by the latency it measures, so the light lands on the beat. It keeps at most one command in flight per bulb and only sends to bulbs you gave a role in the config. Home Assistant only switches the show on and off and stays out of the timing path.

zigbeat is a single static Go binary, meant to run next to Zigbee2MQTT. Architecture, effects, roadmap and decisions: [DESIGN.md](DESIGN.md).

## Measure your network

`zigbeat probe` sends colour commands to a few target bulbs at a rising rate while it watches a far-away **canary** device for the rest of the network. It stops at the first sign of overload: lost commands, rising latency, a slower canary or Zigbee2MQTT errors. The result becomes the engine's budget.

### Install

With Go 1.24 or later:

```bash
go install github.com/siemenvdn/zigbeat/cmd/zigbeat@latest
```

To build for a Raspberry Pi or another machine, see [Build from source](#build-from-source).

### Choose targets and a canary

- **Targets:** two to four colour bulbs (xy colour) that zigbeat will drive later. Mains powered, not behind a switch that might be turned off.
- **Canary:** a mains-powered device as far from the targets as possible, ideally in another room. Not a battery device, because those sleep and answer late. The canary is only read, never changed.

Use Zigbee2MQTT friendly names.

### Run it

Create an MQTT user for zigbeat on your broker. Then, ideally on the same host as the broker:

```bash
export MQTT_USER=zigbeat
read -rs MQTT_PASSWORD && export MQTT_PASSWORD
zigbeat probe -broker 127.0.0.1:1883 -targets pendant-1,pendant-2,pendant-3 -canary hallway-ceiling
```

`zigbeat probe -h` lists all flags.

What you will see:

- The run takes at least 5 minutes, a few more on a strong network. The targets flash in all colours the whole time. Leave them alone, also in Home Assistant.
- At the end every target goes back to its starting state, also after Ctrl-C. A second Ctrl-C quits at once without restoring. The starting state is printed as JSON at the top of the output, so you can restore it by hand.
- Raw samples go to a CSV file.

### Read the result

The probe prints one line per step:

| Column | Meaning |
|---|---|
| `sent`, `ok`, `lost` | Commands sent, confirmed, and unanswered after 5 s |
| `p50`, `p95`, `max` | Time until Zigbee2MQTT confirmed the new state |
| `drift` | Latency kept growing during the step: a queue is building up |
| `canary p95`, `canary lost` | How the rest of the network is doing |
| `z2m err` | Errors Zigbee2MQTT logged during the step |
| `verdict` | `OK`, `WARN` (borderline, repeated once), `STOP` (overloaded) or `PARTIAL` (interrupted) |

The summary gives the highest clean rate, a safe budget (half of it) and the latency at that load.

If `p50` stays below about 20 ms everywhere, Zigbee2MQTT confirmed before the bulb got the command, and the probe measured MQTT instead of the radio. Please [open an issue](https://github.com/siemenvdn/zigbeat/issues) with your output.

## Run the show

### Configure

Copy [`config.example.yaml`](config.example.yaml) to `config.yaml` and fill it in:

- `budget.max_rate`: the suggested budget from the probe. `budget.latency_ms`: the p50 the probe reported at that load. The show will not start without them.
- `fixtures`: your lights, grouped as they hang. Give each fixture a role:
  - `color` for colour bulbs. `mode` decides how a fixture changes: `separate` (one bulb at a time), `together` (the whole fixture at once) or `alternate` (switches between the two every few moments, the default).
  - `breathe` for dimmable bulbs.
- `canary` (optional): a far-away mains-powered device, as for the probe. The show reads it every 5 seconds to watch the rest of the network.
- `effects` (optional): the time between changes, the base brightness, the colour palette and the breathe range.

Lights that are not in the config are never touched. List lights one by one: a Zigbee2MQTT group name is refused, because group commands are broadcasts. The MQTT password comes from the environment variable named in `password_env`.

### Start it

```bash
read -rs ZIGBEAT_MQTT_PASSWORD && export ZIGBEAT_MQTT_PASSWORD
zigbeat run -config config.yaml -bpm 120
```

zigbeat reads the state of every light, prints it as JSON for a manual restore, measures the canary for 5 seconds and starts. Every 10 seconds it prints a status line:

```
21:14:10  moments 35  dropped 2  frames/s 4.2  in flight 1  p50 38 ms  backoff 0  canary 61 ms  z2m err 0
```

| Field | Meaning |
|---|---|
| `moments` | Moments played so far |
| `dropped` | Moments skipped because they could not land on the beat |
| `frames/s` | Zigbee frames sent, always below `budget.max_rate` |
| `p50` | Time until Zigbee2MQTT confirms, per frame |
| `backoff` | 0 normal, 1 no kick, 2 half as many moments, 3 paused |
| `canary`, `z2m err` | Last canary answer and Zigbee2MQTT errors so far |

Ctrl-C, `kill` or a closed SSH session stops the show and puts every light back, lights that were off included. A second Ctrl-C quits at once without restoring.

### When something goes wrong

The show keeps going and gets calmer instead:

- **The network slows down** (rising latency, lost commands, a slow canary, Zigbee2MQTT errors): the backoff level rises one step at a time. At level 3 the show pauses with the lights in their last colour, and resumes after a quiet minute.
- **A light stops answering**, for example switched off at the wall: after two misses zigbeat leaves it alone, reads it once a minute and takes it back when it answers.
- **The broker goes away**: zigbeat reconnects every 5 seconds and carries on. If it is still gone when you stop the show, zigbeat tries for 30 seconds, then prints the saved state of the lights it could not restore and exits with code 1.
- **A light does not answer at the start**: it is left out with a warning; the show runs with the others.

## Home Assistant

`zigbeat serve` keeps running next to Zigbee2MQTT and adds a **zigbeat** device to Home Assistant through MQTT discovery. No Home Assistant YAML is needed, only the MQTT integration that Zigbee2MQTT already uses.

```bash
read -rs ZIGBEAT_MQTT_PASSWORD && export ZIGBEAT_MQTT_PASSWORD
zigbeat serve -config config.yaml -bpm 120
```

It uses the same config as `zigbeat run`. The show is off when serve starts, also after a restart. Switch it on in Home Assistant: zigbeat reads every light, plays, and puts every light back when you switch it off. Until a real beat source exists, the tempo comes from `-bpm`. Ctrl-C, `kill` or a closed SSH session stops serve, after a restore when a show is on. Start it with `nohup` to keep it running after you log out.

| Entity | Shows |
|---|---|
| `switch.zigbeat_show` | Starts and stops the show |
| `sensor.zigbeat_status` | `off`, `starting`, `running`, `paused` or `stopping` |
| `sensor.zigbeat_frames_per_second` | Zigbee frames sent, always below `budget.max_rate` |
| `sensor.zigbeat_latency` | Time until Zigbee2MQTT confirms, per frame |
| `sensor.zigbeat_backoff` | 0 normal, 1 no kick, 2 half as many moments, 3 paused |
| `sensor.zigbeat_lights_playing`, `sensor.zigbeat_lights_left_alone` | Lights in the show, and lights from the config that do not answer |

The numbers follow every 10 seconds and are unknown while the show is off. When zigbeat stops or loses its connection, the entities become unavailable.

Next to the Zigbee2MQTT topics, the MQTT user for zigbeat needs write access to `homeassistant/device/zigbeat/config` and `zigbeat/#`, and read access to `zigbeat/show/set`. Run one `zigbeat serve` per broker, and no `zigbeat run` while its show is on.

To remove the device from Home Assistant, stop zigbeat and clear its discovery message:

```bash
mosquitto_pub -r -n -t homeassistant/device/zigbeat/config
```

## Build from source

```bash
go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o zigbeat ./cmd/zigbeat
```

Try the probe without real lights, against a simulated Zigbee2MQTT that handles about 10 commands per second:

```bash
mosquitto -p 18830 &
go run ./cmd/fake-z2m -broker 127.0.0.1:18830 &
go run ./cmd/zigbeat probe -broker 127.0.0.1:18830 -targets lamp-1,lamp-2,lamp-3 -canary far-lamp \
  -baseline 8s -step 6s -cooldown 2s
```

The probe should report OK up to around 8/s and stop at 10/s.

Run the show against the same simulation with the example config:

```bash
sed 's/1883$/18830/' config.example.yaml > config.yaml
ZIGBEAT_MQTT_PASSWORD=x go run ./cmd/zigbeat run -config config.yaml
```

It should stay at backoff 0 at about 4 frames/s. Restart `fake-z2m` with `-loss 0.1` to watch backoff and lights being left alone, or stop and start `mosquitto` to watch it reconnect.

`zigbeat serve` against the same simulation, with `mosquitto_pub` playing the Home Assistant switch:

```bash
ZIGBEAT_MQTT_PASSWORD=x go run ./cmd/zigbeat serve -config config.yaml &
mosquitto_sub -p 18830 -v -t 'zigbeat/#' &
mosquitto_pub -p 18830 -t zigbeat/show/set -m ON
```

`zigbeat/state` goes from `off` to `starting` and `running`. Publish `OFF` the same way to stop the show and restore the lamps.

Contributor guidelines: [CLAUDE.md](CLAUDE.md).

## License

[GPL-3.0](LICENSE)
