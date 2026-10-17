# Retrotalk TTS for Home Assistant

<img src="retrotalk/logo.png" alt="Retrotalk logo" width="250">

[![Add repository to Home Assistant](https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg)](https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https%3A%2F%2Fgithub.com%2FTom-Neverwinter%2Fretrotalk-wyoming)
[![Import announce blueprint](https://my.home-assistant.io/badges/blueprint_import.svg)](https://my.home-assistant.io/redirect/blueprint_import/?blueprint_url=https%3A%2F%2Fgithub.com%2FTom-Neverwinter%2Fretrotalk-wyoming%2Fblob%2Fmain%2Fblueprints%2Fscript%2Fretrotalk_announce.yaml)

[![CI](https://github.com/Tom-Neverwinter/retrotalk-wyoming/actions/workflows/ci.yaml/badge.svg)](https://github.com/Tom-Neverwinter/retrotalk-wyoming/actions/workflows/ci.yaml)
![Version](https://img.shields.io/badge/dynamic/yaml?url=https%3A%2F%2Fraw.githubusercontent.com%2FTom-Neverwinter%2Fretrotalk-wyoming%2Fmain%2Fretrotalk%2Fconfig.yaml&query=%24.version&label=version)
![Project stage: experimental](https://img.shields.io/badge/project%20stage-experimental-orange)
![Supports aarch64](https://img.shields.io/badge/aarch64-yes-green)
![Supports amd64](https://img.shields.io/badge/amd64-yes-green)
![Wyoming protocol](https://img.shields.io/badge/wyoming-TTS%20%2B%20streaming-blue)
[![License](https://img.shields.io/github/license/Tom-Neverwinter/retrotalk-wyoming?cacheSeconds=3600)](LICENSE)

> **🚧 Work in progress.** The engine is new and still being tuned. It's tested end to end with
> Home Assistant's Wyoming client library in CI, but not yet on a real Home Assistant install.
> Issues and PRs welcome.

Retrotalk is a local text-to-speech add-on with the robotic charm of 1980s formant
synthesizers. The speech engine is written from scratch in Go and runs entirely on your Home
Assistant machine: no cloud, no internet, no external programs.

- 9 built-in voices, plus **your own custom voices** set in the add-on's Configuration tab
- Bundled impression voices: `kitt` (default) and `hal`
- Streaming: each phrase plays as soon as it's ready; accepts streamed text from voice assistants
- Reads numbers, times, temperatures, money, percentages and abbreviations naturally
- Auto-discovered by Home Assistant's Wyoming integration
- **Announce blueprint:** speak everywhere, in chosen rooms, or on chosen speakers, with one
  designated speaker per room, plus mute switch, quiet hours, priority alerts and volume
- English only for now

## Install

1. Click the **Add repository** button above, or in Home Assistant go to
   **Settings → Add-ons → Add-on Store → ⋮ → Repositories** and add:
   `https://github.com/Tom-Neverwinter/retrotalk-wyoming`
2. Install **Retrotalk TTS** and start it.
3. Accept the discovered **Wyoming Protocol** integration under **Settings → Devices & services**.

Full docs (options, making custom voices, announcements) are in
[`retrotalk/DOCS.md`](retrotalk/DOCS.md), which is also the add-on's **Documentation** tab.

## How the engine works

| Stage | What it does | Based on |
|---|---|---|
| Text normalization | Numbers, times, °F, $, %, ordinals, abbreviations into words | — |
| Pronunciation | Dictionary lookup, letter-to-sound rules for unknown words | [CMU Pronouncing Dictionary](https://github.com/cmusphinx/cmudict) |
| Prosody | Segment durations, stress, hat-pattern pitch contour | Klatt's duration rules (Allen, Hunnicutt & Klatt 1987) |
| Synthesis | Cascade/parallel formant synthesizer, 16 kHz | Klatt (1980); Klatt & Klatt (1990) voicing source |

Intelligibility is measured with Whisper (speech recognition) on 20 Harvard sentences plus
10 Home Assistant announcements: about **18-32% word error rate** depending on the voice,
against 2% for a modern commercial voice. It's understandable, and very retro.

## Repository layout

| Path | What |
|---|---|
| `retrotalk/tts/` | The speech engine (Go, standard library only) |
| `retrotalk/main.go` | Wyoming server: voices, streaming, Home Assistant discovery |
| `retrotalk/config.yaml` | Add-on manifest and options schema |
| `retrotalk/translations/en.yaml` | UI labels for the options |
| `retrotalk/Dockerfile` | One static binary on an empty base image |
| `blueprints/script/retrotalk_announce.yaml` | Room/speaker announcement script blueprint |
| `.github/workflows/ci.yaml` | Lint, unit tests, build both architectures, smoke test with the Wyoming client (WAVs uploaded as artifacts), publish images |

Run the tests with `cd retrotalk && go test ./...`.

## Status / To-do

- [x] Own speech engine: text normalization, CMUdict + letter-to-sound, prosody, formant synthesis
- [x] Wyoming `describe` / `synthesize` and streaming text in, phrase-by-phrase audio out
- [x] UI options: default voice, rate, custom voices
- [x] Wyoming auto-discovery via the Supervisor
- [x] Announce blueprint: all / areas / speakers, mute switch, quiet hours, priority, volume
- [x] CI: amd64 and aarch64 builds, smoke test, prebuilt images
- [ ] Verified on a Raspberry Pi 4 running Home Assistant
- [ ] Better intelligibility: velar consonants, R vs L, female and child voices
- [ ] More natural intonation (question and list contours, emphasis)
- [ ] Other languages (letter-to-sound rules for Spanish and German are fairly regular)

## History

Versions up to 0.5.0 wrapped the original DECtalk engine. Its licence is proprietary, so 0.6.0
replaced it with this engine written from published research, and the project was renamed.
The old images containing DECtalk were withdrawn.

## Licensing

Everything in this repository is MIT licensed (see `LICENSE`), except the embedded CMU
Pronouncing Dictionary in `retrotalk/tts/cmudict.txt.gz`, which is © Carnegie Mellon University
under a BSD licence (see `retrotalk/tts/CMUDICT-LICENSE`, also shipped inside the image).
