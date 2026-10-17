# DECtalk TTS for Home Assistant

<img src="dectalk/logo.png" alt="DECtalk TTS logo" width="250">

[![Add repository to Home Assistant](https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg)](https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https%3A%2F%2Fgithub.com%2FTom-Neverwinter%2Fdectalk-wyoming)
[![Import announce blueprint](https://my.home-assistant.io/badges/blueprint_import.svg)](https://my.home-assistant.io/redirect/blueprint_import/?blueprint_url=https%3A%2F%2Fgithub.com%2FTom-Neverwinter%2Fdectalk-wyoming%2Fblob%2Fmain%2Fblueprints%2Fscript%2Fdectalk_announce.yaml)

[![CI](https://github.com/Tom-Neverwinter/dectalk-wyoming/actions/workflows/ci.yaml/badge.svg)](https://github.com/Tom-Neverwinter/dectalk-wyoming/actions/workflows/ci.yaml)
![Version](https://img.shields.io/badge/dynamic/yaml?url=https%3A%2F%2Fraw.githubusercontent.com%2FTom-Neverwinter%2Fdectalk-wyoming%2Fmain%2Fdectalk%2Fconfig.yaml&query=%24.version&label=version)
![Project stage: experimental](https://img.shields.io/badge/project%20stage-experimental-orange)
![Supports aarch64](https://img.shields.io/badge/aarch64-yes-green)
![Supports amd64](https://img.shields.io/badge/amd64-yes-green)
![Wyoming protocol](https://img.shields.io/badge/wyoming-TTS%20%2B%20streaming-blue)
[![License](https://img.shields.io/github/license/Tom-Neverwinter/dectalk-wyoming?cacheSeconds=3600)](LICENSE)

> **🚧 Work in progress.** The Wyoming server is tested against the Python `wyoming`
> client library, but the full add-on has not yet been built and run on real Home Assistant
> hardware. Expect rough edges. Issues and PRs welcome.

A Home Assistant add-on that runs the classic [DECtalk](https://github.com/dectalk/dectalk)
speech synthesizer as a local [Wyoming](https://github.com/rhasspy/wyoming) text-to-speech
server. No cloud, no internet needed once installed.

- 9 built-in DECtalk voices, plus **your own custom voices** set in the add-on's Configuration tab
- Bundled impression voices: `kitt` (default) and `hal`
- Streaming: audio starts playing before the whole message is synthesized
- Default voice, speaking rate and languages configurable in the UI
- 6 languages: US and UK English, German, Spanish (Spain and Latin America), French
- Auto-discovered by Home Assistant's Wyoming integration
- **Announce blueprint:** speak everywhere, in chosen rooms, or on chosen speakers, with one
  designated speaker per room, plus mute switch, quiet hours, priority alerts and volume

## Install

1. Click the **Add repository** button above, or in Home Assistant go to
   **Settings → Add-ons → Add-on Store → ⋮ → Repositories** and add:
   `https://github.com/Tom-Neverwinter/dectalk-wyoming`
2. Install **DECtalk TTS** and start it. The first install compiles DECtalk from source on
   your device, so it can take several minutes on a Raspberry Pi.
3. Accept the discovered **Wyoming Protocol** integration under **Settings → Devices & services**.

Full docs (options, making custom voices, announcements) are in
[`dectalk/DOCS.md`](dectalk/DOCS.md), which is also the add-on's **Documentation** tab.

## Repository layout

| Path | What |
|---|---|
| `dectalk/main.go` | Wyoming server in Go (stdlib only). Pipes text into DECtalk's `say` and streams its raw PCM back as it's produced. |
| `dectalk/config.yaml` | Add-on manifest and options schema |
| `dectalk/translations/en.yaml` | UI labels for the options |
| `dectalk/Dockerfile` | Builds DECtalk from upstream and checks it can produce a WAV |
| `blueprints/script/dectalk_announce.yaml` | Room/speaker announcement script blueprint |
| `.github/workflows/ci.yaml` | Lint, unit tests, then build both architectures and smoke-test real DECtalk (WAVs uploaded as artifacts) |

Run the tests with `cd dectalk && go test ./...`.

## Status / To-do

- [x] Wyoming `describe` / `synthesize`, tested with `wyoming==1.5.4` client
- [x] Builds for `amd64` and `aarch64` (Go side)
- [x] UI options: default voice, rate, custom voices
- [x] Wyoming auto-discovery via the Supervisor
- [x] Announcement blueprint (all / areas / speakers)
- [x] DECtalk builds and speaks on amd64 and aarch64 (CI smoke test with real DECtalk)
- [ ] Verified on a Raspberry Pi 4 running Home Assistant
- [ ] Verified discovery and blueprint on a live Home Assistant
- [ ] Prebuilt images (currently builds on your device)
- [x] Pin the DECtalk source to a known-good commit (`DECTALK_REF` build arg)
- [x] Streaming audio out (no temp files) and streaming text in (`synthesize-start` / `synthesize-chunk`)
- [x] Bundled `kitt` (default) and `hal` impression voices
- [ ] Tune `kitt` / `hal` by ear on real DECtalk output
- [ ] Keep DECtalk resident (cgo/libtts) instead of one process per message
- [x] Other DECtalk languages (UK English, German, Spanish, Latin American Spanish, French)
- [x] Announce blueprint: mute switch, quiet hours, priority, volume with restore, no overlap

## Licensing

The add-on code in this repo is MIT licensed (see `LICENSE`). **DECtalk itself is not part of
this repo.** It is downloaded and compiled from [dectalk/dectalk](https://github.com/dectalk/dectalk)
when you install the add-on, and its licensing status is unclear. Check it before redistributing
built images.
