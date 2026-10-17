# DECtalk TTS for Home Assistant

> **🚧 Work in progress.** The Wyoming server is tested against the Python `wyoming`
> client library, but the full add-on has not yet been built and run on real Home Assistant
> hardware. Expect rough edges. Issues and PRs welcome.

A Home Assistant add-on that runs the classic [DECtalk](https://github.com/dectalk/dectalk)
speech synthesizer as a local [Wyoming](https://github.com/rhasspy/wyoming) text-to-speech
server. No cloud, no internet needed once installed.

- 9 built-in DECtalk voices, plus **your own custom voices** set in the add-on's Configuration tab
- Default voice and speaking rate configurable in the UI
- Auto-discovered by Home Assistant's Wyoming integration
- **Announce blueprint:** speak everywhere, in chosen rooms, or on chosen speakers, with one
  designated speaker per room

## Install

1. In Home Assistant, go to **Settings → Add-ons → Add-on Store → ⋮ → Repositories** and add:
   `https://github.com/Tom-Neverwinter/dectalk-wyoming`
2. Install **DECtalk TTS** and start it. The first install compiles DECtalk from source on
   your device, so it can take several minutes on a Raspberry Pi.
3. Accept the discovered **Wyoming Protocol** integration under **Settings → Devices & services**.

Full docs (options, making custom voices, announcements) are in
[`dectalk/DOCS.md`](dectalk/DOCS.md), which is also the add-on's **Documentation** tab.

## Repository layout

| Path | What |
|---|---|
| `dectalk/main.go` | Wyoming server in Go (stdlib only). Runs DECtalk's `say` per request and streams the PCM back. |
| `dectalk/config.yaml` | Add-on manifest and options schema |
| `dectalk/translations/en.yaml` | UI labels for the options |
| `dectalk/Dockerfile` | Builds DECtalk from upstream and checks it can produce a WAV |
| `blueprints/script/dectalk_announce.yaml` | Room/speaker announcement script blueprint |

Run the tests with `cd dectalk && go test ./...`.

## Status / To-do

- [x] Wyoming `describe` / `synthesize`, tested with `wyoming==1.5.4` client
- [x] Builds for `amd64` and `aarch64` (Go side)
- [x] UI options: default voice, rate, custom voices
- [x] Wyoming auto-discovery via the Supervisor
- [x] Announcement blueprint (all / areas / speakers)
- [ ] Verified full add-on build on a Raspberry Pi 4
- [ ] Verified discovery and blueprint on a live Home Assistant
- [ ] Prebuilt images (currently builds on your device)
- [ ] Pin the DECtalk source to a known-good commit
- [ ] Keep DECtalk resident (cgo/libtts) instead of one process per request
- [ ] Streaming synthesis (`synthesize-start` / `synthesize-chunk`)
- [ ] Other DECtalk languages (Spanish, German, …)

## Licensing

The add-on code in this repo is MIT licensed (see `LICENSE`). **DECtalk itself is not part of
this repo.** It is downloaded and compiled from [dectalk/dectalk](https://github.com/dectalk/dectalk)
when you install the add-on, and its licensing status is unclear. Check it before redistributing
built images.
