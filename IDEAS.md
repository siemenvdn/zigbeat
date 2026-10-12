# Ideas

Ideas for later, grouped by topic. Nothing here is planned yet; plans live in the [roadmap](DESIGN.md#roadmap), small known issues in [BACKLOG.md](BACKLOG.md).

## Calibration

- **A click track through Music Assistant.** zigbeat generates a click track at a few tempos (for example 90, 124 and 172 BPM), serves it over HTTP and has Music Assistant play it on the speaker through its API, with the same token. The sound takes exactly the path real music takes, and the analysis finds perfect beats in clicks. One bulb flashes on every click (2 frames/s at 120 BPM, within the budget) and the user moves the offset with `+` and `-` until click and flash coincide, or taps along with the clicks. Pressing one key on hearing and another on seeing measures reaction times instead, which differ by tens of ms between hearing and seeing: about the offset being measured.

## Setup

- **Fixtures from Zigbee2MQTT groups.** A group whose members are all offered lights is a fixture candidate. zigbeat would still never send to the group.
- **Bulbs with only `color_hs` as `color`,** by sending `{"color":{"hue":...,"saturation":...}}` instead of hex.
- **Lights with several channels,** such as a Gledopto controller with an RGB and a white channel, as two lights with suffixed properties (`state_rgb`, `state_white`).
- **An override for a bulb the allowlist refuses wrongly,** such as a per-light `allow: true` with a loud warning. Only when a real case shows up.
- **Setup proposes a `music_assistant` block,** found with mDNS or asked for.
- **`zigbeat setup -devices devices.json`** classifies a saved device list without a broker, for support.

## Effects

- **Energy that steers the effects:** how many bulbs change, how high the kick goes, which variant plays, how deep the breathe bulbs go, and a burst at a drop (energy rising by 0.5 or more between bars). Never more than three messages per moment. First see energy on real music: `zigbeat beats` prints it.
- **A random variant per phrase**, never the same twice in a row, in place of the fixed rotation (`rotation` in `internal/run/show/rotation.go`).
- **Pale fill-ins:** when a cover has fewer hues than `album_colors`, pale versions of its hues instead of other hues, as Music Assistant's Hue plugin does.
- **A section event** for a detected drop or breakdown, next to `Track` and `Beat` in the music stream.
- **Colour swatches in `zigbeat beats`** (ANSI 24-bit background colours), to compare the colours with the cover at a glance.
- **Now playing in Home Assistant:** title and artist as fields of `Track`, from Music Assistant's main API.
- **Loudness from a Sendspin visualizer** (`visualizer@v1` types `loudness`, `peak` and `spectrum`), or from Music Assistant's `audio_analysis` API (1800 bins of `rms_energy` and `band_rms` per track), if the relay's waveform frames are not enough for energy.
