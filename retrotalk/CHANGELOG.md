# Changelog

## Announce blueprint update (no add-on update needed)

- New "Speaker wake-up lead-in" (default 1000 ms): silence before each announcement, so
  speakers that wake slowly (Sonos) no longer cut off the first words.
- The volume is restored only after playback has really started and finished; before, a
  speaker slow to report "playing" got its volume restored mid-announcement.
  Re-import the blueprint to get it.

## 0.7.1

Security and robustness, found by attacking the add-on on purpose:
- Fixed a crash: a malformed amount like "$1..2" in a Spanish, German or French message
  could take the whole add-on down. A crash in one message can no longer stop the add-on.
- Hostile or runaway messages are bounded: sung notes up to 3 s, words up to 40 letters,
  phrases split after 40 words, [:dial] up to 64 keys, 10 minutes of audio per message.
  An over-long message is spoken up to the limit and then ends normally.
- Connections: at most 32 at once; silent ones are dropped after 10 s, idle streams after
  2 minutes, and clients that stop reading after 30 s.
- Streamed text without punctuation is spoken as it arrives instead of piling up.
- Tones, pauses and dial tones are sent as they are made instead of built up in memory.
- Faster Spanish, German and French pronunciation on long words.
- Fuzz tests and the race detector now run in CI.

## 0.7.0

- **Much clearer speech.** Engine constants tuned automatically against speech recognizers,
  plus fixes for "-ing" endings and z/v/th sounds: word errors on unseen sentences fell from
  32% to 17%, and every language benefits
- **Intonation:** yes/no questions rise, wh-questions fall, lists rise between items, later
  clauses step down, exclamations are livelier
- **Languages:** UK English, Spanish (Spain and Latin America), German and French (experimental),
  enabled under the new Languages option; custom voices take a `language`
- **Character voices** fitted to measurements of reference recordings: `kitt`, `hal`, `glados`,
  `turret`, `jarvis`, `narrator`, plus `robot`; `kit` is softer
- **Inline commands:** `[:rate]`, `[:volume]`, `[:name]` voice switching, `[:dv]`, `[:lang]`,
  pauses, spell mode, reading punctuation, tones and telephone dialing
- **Phonemic input and singing:** `[hh ah1 l ow]`, `[d ey<600,32>]`
- **Pronunciations** option to fix names and words
- **Emphasis** with `*asterisks*`
- New voice settings: `na` nasality, `pq` pitch steps (autotune), and DECtalk's `bf` `hr` `sr`
  `gv` `gh` `gf` `gn` `f4` `b4` `f5` `b5` `lx` `nf`
- Fixed: high voices were louder than low ones; "-5°F" now reads "minus five degrees"

## 0.6.0

- **New engine, new name.** DECtalk's licence is proprietary, so it's replaced by Retrotalk,
  a formant speech engine written from scratch from published research (Klatt), with
  pronunciation from the CMU Pronouncing Dictionary. Everything is now redistributable.
- Prebuilt images: installs download in seconds instead of compiling on the device
- Audio is 16 kHz (was 11 kHz) and streams a phrase at a time
- English only for now; the DECtalk language voices (`paul_gr`, ...) are gone
- DECtalk inline commands (`[:...]`) in messages are ignored
- Add-on slug is now `retrotalk`: install it fresh rather than updating the old add-on

## 0.5.0

- Languages: UK English, German, Spanish (Spain and Latin America) and French built-in voices
  (`paul_gr`, `betty_fr`, ...), enabled in the Configuration tab; custom voices take a language
- Announce blueprint: mute switch, quiet hours, priority override, per-announcement volume
  with restore, and queued announcements that wait for each other instead of overlapping
- CI builds and smoke-tests real DECtalk on amd64 and aarch64 for every change (images are
  not published because DECtalk's licence is proprietary)

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
