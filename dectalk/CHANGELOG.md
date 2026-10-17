# Changelog

## 0.4.1

- Non-ASCII text (°, é, curly quotes) is now spoken instead of silently dropped
- DECtalk source pinned to a known commit; build self-test matches how the server runs `say`
- A stuck `say` process is stopped after 5 minutes
- Announce blueprint also finds labels set on devices
- Store icon, logo and experimental tag

## 0.4.0

- Streaming: audio is sent as DECtalk produces it, with no temp files
- Accepts streamed text from Home Assistant voice assistants (`synthesize-start` / `-chunk` / `-stop`)
- New bundled voices: `kitt` (now the default) and `hal`
- Fixed a crash on messages longer than 4 KB

## 0.3.0

- Configuration tab options: default voice, speaking rate, custom voices
- Auto-discovery by the Wyoming integration
- Host port closed by default (Home Assistant connects internally)
- Announce script blueprint: everywhere, by room, or by speaker

## 0.2.0

- Wyoming server rewritten in Go (single static binary, no Python)

## 0.1.0

- First version: DECtalk as a Wyoming TTS server (Python)
