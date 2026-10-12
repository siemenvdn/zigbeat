# Ideas

Ideas for later, grouped by topic. Nothing here is planned yet; plans live in the [roadmap](DESIGN.md#roadmap), small known issues in [BACKLOG.md](BACKLOG.md).

## Calibration

- **A click track through Music Assistant.** zigbeat generates a click track at a few tempos (for example 90, 124 and 172 BPM), serves it over HTTP and has Music Assistant play it on the speaker through its API, with the same token. The sound takes exactly the path real music takes, and the analysis finds perfect beats in clicks. One bulb flashes on every click (2 frames/s at 120 BPM, within the budget) and the user moves the offset with `+` and `-` until click and flash coincide, or taps along with the clicks. Pressing one key on hearing and another on seeing measures reaction times instead, which differ by tens of ms between hearing and seeing: about the offset being measured.

## Setup

- **Fixtures from Zigbee2MQTT groups.** A group whose members are all offered lights is a fixture candidate. zigbeat would still never send to the group.
- **Fixtures from a shared name.** setup groups by a trailing number only, so `hall-pendant_top`, `hall-pendant_center` and `hall-pendant_bottom` become three one-bulb fixtures. Lights whose names differ in one word could be proposed as one fixture.
- **Bulbs with only `color_hs` as `color`,** by sending `{"color":{"hue":...,"saturation":...}}` instead of hex.
- **Lights with several channels,** such as a Gledopto controller with an RGB and a white channel, as two lights with suffixed properties (`state_rgb`, `state_white`).
- **An override for a bulb the allowlist refuses wrongly,** such as a per-light `allow: true` with a loud warning. Only when a real case shows up.
- **Setup proposes a `music_assistant` block,** found with mDNS or asked for.
- **`zigbeat setup -devices devices.json`** classifies a saved device list without a broker, for support.

## Effects

- **Energy that steers the effects,** within the room of a moment and the breathe range. Built: a quiet part plays calmer (`follow_energy`). Still ideas: how high the kick goes, which variant plays, how deep the breathe bulbs go, and a burst at a drop (energy rising by 0.5 or more between bars). `zigbeat beats` prints the energy per beat.
- **An effect per energy level.** In a quiet part every colour bulb takes one colour together and fades slowly over the whole moment; at full energy the variants as now; maybe a burst at a drop. The user's wish after the first evening with Music Assistant.
- **Kicks off the beat grid,** for hip-hop and drum and bass, whose kicks are not on every beat: onsets in the low band of the waveform frames, which arrive 5 s ahead, scheduled like the moments and within their room. Sendspin's `peak` will not do: it fires on any transient. Single hi-hats are out of reach: sixteenths at 125 BPM are 8 a second, more than a bulb shows and than the budget holds for the whole room. How busy the hi-hats are could steer the effects like the energy does.
- **A random variant per phrase**, never the same twice in a row, in place of the fixed rotation (`rotation` in `internal/run/show/rotation.go`).
- **Pale fill-ins:** when a cover has fewer hues than `album_colors`, pale versions of its hues instead of other hues, as Music Assistant's Hue plugin does.
- **A section event** for a detected drop or breakdown, next to `Track` and `Beat` in the music stream.
- **Colour swatches in `zigbeat beats`** (ANSI 24-bit background colours), to compare the colours with the cover at a glance.
- **Now playing in Home Assistant:** title and artist as fields of `Track`, from Music Assistant's main API.
- **Loudness from Music Assistant's `audio_analysis` API** (1800 bins of `rms_energy` and `band_rms` per track), if the relay's waveform frames are not enough for energy.

## Beat source

- **Sendspin as a second `music.Source`,** for time stamps on the real playback clock of MA's web player and AirPlay speakers, when the offset turns out to differ per track there, or when the relay goes. Only with MA's Smart Fades beats as Sendspin `beat`; why it waits is under Rejected in [DESIGN.md](DESIGN.md#beat-sources).

## Home Assistant

- **A button "back to the config"** that wipes every kept choice: an empty retained message on each style topic.
- **The phase 1b sensors as diagnostic entities** (`entity_category: diagnostic`), so the switch and the style stand alone at the top of the device page.
- **The palette or the fixture mode from Home Assistant.**
- **A sensor with the variant that plays now,** from the planner's variant note.
- **Presets per kind of party,** such as "dinner" and "dance": a select that sets several choices at once.

## Interface

- **A settings page** for everything in the config: the lights and fixtures, their roles and modes, which effect plays at which energy, the palette. Served by `zigbeat serve` next to Zigbee2MQTT, writing `config.yaml`; Home Assistant keeps the knobs for during the show. First the effect model has to settle: what can be set per energy level and per fixture.
