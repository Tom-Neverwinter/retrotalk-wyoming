# DECtalk TTS for Home Assistant

> **🚧 Work in progress.** The Wyoming server is tested against the Python `wyoming`
> client library, but the full add-on has not yet been built and run on real Home Assistant
> hardware. Expect rough edges. Issues and PRs welcome.

A Home Assistant add-on that runs the classic [DECtalk](https://github.com/dectalk/dectalk)
speech synthesizer as a local [Wyoming](https://github.com/rhasspy/wyoming) text-to-speech
server. No cloud, no internet needed once installed.

## Install

1. In Home Assistant, go to **Settings → Add-ons → Add-on Store → ⋮ → Repositories** and add:
   `https://github.com/Tom-Neverwinter/dectalk-wyoming`
2. Install **DECtalk TTS** and start it. The first install compiles DECtalk from source on
   your device, so it can take several minutes on a Raspberry Pi.
3. Go to **Settings → Devices & services → Add integration → Wyoming Protocol** and enter
   host `local-dectalk` (or your HA IP) and port `10205`.
   *(Auto-discovery isn't implemented yet. See the To-do list.)*
4. Pick DECtalk under **Settings → Voice assistants**, or call `tts.speak`.

## Voices

`paul` (default), `betty`, `harry`, `frank`, `dennis`, `kit`, `ursula`, `rita`, `wendy`

DECtalk inline commands work in the text, e.g. `[:rate 250]` or `[:phoneme on]`.

## How it works

- `dectalk/main.go` is a small Go Wyoming server using only the standard library. For each
  `synthesize` request it runs DECtalk's `say` to a temp WAV file and streams the PCM back
  as `audio-start` / `audio-chunk` / `audio-stop`.
- The Dockerfile builds DECtalk from upstream source and checks that it can produce a WAV
  before the image is finished.

Run the tests with `cd dectalk && go test ./...`.

## Status / To-do

- [x] Wyoming `describe` / `synthesize`, tested with `wyoming==1.5.4` client
- [x] Builds for `amd64` and `aarch64` (Go side)
- [ ] Verified full add-on build on a Raspberry Pi 4
- [ ] Wyoming auto-discovery via the Supervisor API
- [ ] Prebuilt images (currently builds on your device)
- [ ] Pin the DECtalk source to a known-good commit
- [ ] Streaming synthesis (`synthesize-start` / `synthesize-chunk`)
- [ ] Other DECtalk languages (Spanish, German, …)

## Licensing

The add-on code in this repo is MIT licensed (see `LICENSE`). **DECtalk itself is not part of
this repo.** It is downloaded and compiled from [dectalk/dectalk](https://github.com/dectalk/dectalk)
when you install the add-on, and its licensing status is unclear. Check it before redistributing
built images.
