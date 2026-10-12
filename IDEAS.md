# Ideas

Ideas for later, grouped by topic. Nothing here is planned yet; plans live in the [roadmap](DESIGN.md#roadmap), small known issues in [BACKLOG.md](BACKLOG.md).

## Calibration

- **A click track through Music Assistant.** zigbeat generates a click track at a few tempos (for example 90, 124 and 172 BPM), serves it over HTTP and has Music Assistant play it on the speaker through its API, with the same token. The sound takes exactly the path real music takes, and the analysis finds perfect beats in clicks. One bulb flashes on every click (2 frames/s at 120 BPM, within the budget) and the user moves the offset with `+` and `-` until click and flash coincide, or taps along with the clicks. Pressing one key on hearing and another on seeing measures reaction times instead, which differ by tens of ms between hearing and seeing: about the offset being measured.
