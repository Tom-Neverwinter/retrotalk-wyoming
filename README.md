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

> **🚧 Work in progress.** The engine is new and still being tuned. It runs on a Raspberry Pi 4
> with Home Assistant 2026.10, but the announce blueprint hasn't been tried on real speakers yet.
> Issues and PRs welcome.

Retrotalk is a local text-to-speech add-on with the robotic charm of 1980s formant
synthesizers. The speech engine is written from scratch in Go and runs entirely on your Home
Assistant machine: no cloud, no internet, no external programs.

- **Six languages:** US and UK English, Spanish (Spain and Latin America), German, French
- 9 built-in voices per language, plus **your own custom voices** set in the Configuration tab
- **Character voices** fitted to measurements of reference recordings: `kitt` (default), `hal`,
  `glados`, `turret`, `jarvis`, `narrator`, `robot`
- **DECtalk-style inline commands:** `[:rate]`, `[:name hal]`, `[:dv ap 90]`, `[:lang sp]`,
  pauses, spelling, tones and telephone dialing; **phonemic input and singing**
- **Pronunciations** option to fix names and words; `*emphasis*`
- 26 voice settings (DECtalk's `[:dv]` codes plus nasality and autotune pitch steps)
- Streaming: each phrase plays as soon as it's ready; accepts streamed text from voice assistants
- Reads numbers, times, temperatures, money, percentages and abbreviations naturally
- Auto-discovered by Home Assistant's Wyoming integration
- **Announce blueprint:** speak everywhere, in chosen rooms, or on chosen speakers, with one
  designated speaker per room, plus mute switch, quiet hours, priority alerts and volume

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
| Text normalization | Numbers, times, °F, $, %, ordinals, abbreviations into words, per language | — |
| Pronunciation | English: dictionary lookup, letter-to-sound rules for unknown words; UK English as a transform of it. Spanish, German, French: spelling rules | [CMU Pronouncing Dictionary](https://github.com/cmusphinx/cmudict) |
| Prosody | Segment durations, stress, hat-pattern pitch contour | Klatt's duration rules (Allen, Hunnicutt & Klatt 1987) |
| Synthesis | Cascade/parallel formant synthesizer, 16 kHz | Klatt (1980); Klatt & Klatt (1990) voicing source |

Intelligibility is measured by having speech recognizers transcribe it: NVIDIA Parakeet
models (Phonon-2, Redux) run with [audio.cpp](https://github.com/0xShug0/audio.cpp), and
Whisper. The engine's acoustic constants (`tts/tune.go`) were tuned automatically against
150 LibriSpeech sentences and checked on 150 others it never saw:

| | Word error rate |
|---|---|
| English, held-out sentences, mean of 3 recognizers | **17%** (32% before tuning; about 50% at the start) |
| All 9 built-in English voices | 14-24% |
| Spanish (Spain / Latin America) | 15-17% |
| German | 17-37% |
| French | 50-73% (experimental) |
| A modern commercial voice, for reference | 2% |

Non-English numbers use multilingual Whisper, which is more forgiving than the Parakeet
models. It's understandable, and very retro.

## Repository layout

| Path | What |
|---|---|
| `retrotalk/tts/` | The speech engine (Go, standard library only): `klatt.go` synthesizer, `prosody.go`, `text.go`, `lexicon.go`, `lang.go` languages, `commands.go` inline commands, `tune.go` tuned constants |
| `retrotalk/cmd/preview/` | Local web page to listen to voices and settings: `cd retrotalk && go run ./cmd/preview`, then open http://localhost:8099 |
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
- [x] Installed on a Raspberry Pi 4 (Home Assistant 2026.10.2): prebuilt image installs in
  seconds, auto-discovery, all 12 voices, speech generated through `tts.retrotalk`;
  about 21 MB of memory
- [ ] Announce blueprint verified on real speakers
- [x] Engine constants tuned automatically against speech recognizers (held-out checked)
- [x] Velar consonants and R vs L (now among the clearest sounds); female and child voices
  as clear as male
- [x] Intonation: yes/no questions rise, wh-questions fall, list continuations, downstep,
  livelier exclamations, `*emphasis*`
- [x] Languages: UK English, Spanish x2, German, French; character voices; DECtalk-style
  commands, phonemes and singing; pronunciations; 26 voice settings
- [ ] French: still experimental (about half of words misrecognized)
- [ ] Hardest remaining sounds: th (heard as f), ch/jh, the -ing ending

## History

Versions up to 0.5.0 wrapped the original DECtalk engine. Its licence is proprietary, so 0.6.0
replaced it with this engine written from published research, and the project was renamed.
The old images containing DECtalk were withdrawn.

## Licensing

Everything in this repository is MIT licensed (see `LICENSE`), except the embedded CMU
Pronouncing Dictionary in `retrotalk/tts/cmudict.txt.gz`, which is © Carnegie Mellon University
under a BSD licence (see `retrotalk/tts/CMUDICT-LICENSE`, also shipped inside the image).
