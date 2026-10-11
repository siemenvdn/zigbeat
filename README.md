# zigbeat

**Make your Zigbee lights move with your music, without flooding your network.**

Ordinary Zigbee bulbs on [Zigbee2MQTT](https://www.zigbee2mqtt.io/) have no good way to follow music. Spotify closed its audio analysis API, and blasting commands at a few bulbs fills the shared mesh until the lights lag and the rest of the house slows down. zigbeat does the opposite of a strobe: fewer changes, always on the beat, within a budget measured on your own network.

## Status

- **Works today:** `zigbeat probe` measures how many light commands per second your network handles.
- **In progress:** the light engine, first driven by a fixed-BPM metronome.
- **Later:** a real beat source, `zigbeat setup`, a container image.

## What it will do

- Colour bulbs change colour on the beat, every 2 to 5 seconds.
- Dimmable bulbs breathe slowly with the music.
- It stays below the budget the probe found and backs off when the network gets busy.
- Every light goes back to how it was when the show stops, also after Ctrl-C or a crash.
- Home Assistant gets an on/off switch through MQTT discovery, without any YAML.

It does not flash on every beat, use Zigbee groups, or switch relays and plugs. [DESIGN.md](DESIGN.md) explains why.

## How it works

```
 beat source ──(beat: audible time, bpm, position in bar, energy)──▶ zigbeat engine ──MQTT──▶ Zigbee2MQTT ──▶ bulbs
                                                                         ▲   │
                                          on/off and status (MQTT discovery)  │ snapshot before, restore after
                                                                         │   ▼
                                                                  Home Assistant
```

A beat source says when each beat becomes audible. The engine picks a few bulbs per moment and sends each command early by the latency the probe measured, so the light lands on the beat. It keeps at most one command in flight per bulb and only sends to bulbs you gave a role in the config. Home Assistant only switches the show on and off and stays out of the timing path.

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

Contributor guidelines: [CLAUDE.md](CLAUDE.md).

## License

[GPL-3.0](LICENSE)
