# Retrotalk TTS

> ✅ Working, still being tuned. See the [repository](https://github.com/Tom-Neverwinter/retrotalk-wyoming)
> for status.

Retrotalk is a local, retro-sounding text-to-speech engine in the style of 1980s formant
synthesizers. It runs entirely on your Home Assistant machine, with no cloud and no internet.
It speaks US and UK English, Spanish (Spain and Latin America), German and French.

## Setup

1. Start the add-on.
2. Home Assistant should show **Wyoming Protocol** under **Settings → Devices & services →
   Discovered**. Click **Configure**.
   - Not discovered? Add **Wyoming Protocol** manually with the **Hostname** shown on this
     add-on's Info tab and port `10205`.
3. Pick Retrotalk in **Settings → Voice assistants**, or use the `tts.speak` action.

## Options

| Option | What it does |
|---|---|
| **Default voice** | Voice used when a request doesn't name one. Any built-in or custom voice name. |
| **Speaking rate** | Words per minute for all voices (75-600). Unset = each voice's own rate. |
| **Languages** | Which languages get the 9 built-in voices (see [Languages](#languages)). Default: US English. |
| **Custom voices** | Your own voices (below). They show up in Home Assistant's voice pickers. |
| **Pronunciations** | Fix how words are said (see [Pronunciations](#pronunciations)). |

Restart the add-on after changing options, then **reload** the Wyoming integration so
Home Assistant sees new voices.

## Built-in voices

| Voice | Sounds like |
|---|---|
| `paul` | Standard adult male |
| `harry` | Large, deep male |
| `frank` | Older male, a little breathy and creaky |
| `dennis` | Softer, breathier male |
| `betty` | Standard adult female |
| `ursula` | Assertive, bright female |
| `rita` | Gravelly female |
| `wendy` | Whispery female |
| `kit` | Child |

## Bundled character voices

These ship as defaults in **Custom voices**, so you can edit or delete them there. They are
*impressions* of fictional voices, fitted to measurements of reference recordings (pitch,
pitch range, speaking rate, pitch jumps), not recordings or clones.

| Voice | Inspired by | Character |
|---|---|---|
| `kitt` (**default**) | KITT, *Knight Rider* | Crisp, precise, quick, a hint of nasal |
| `hal` | HAL 9000, *2001: A Space Odyssey* | Calm, low, smooth, natural pace |
| `glados` | GLaDOS, *Portal* | Bright, clean, autotuned pitch steps |
| `turret` | Sentry turret, *Portal* | High, sweet, slow, autotuned |
| `jarvis` | J.A.R.V.I.S., *Iron Man* | Brisk, precise, butler-like |
| `narrator` | A classic documentary narrator | Deep, warm, unhurried |
| `robot` | Classic monotone robot | Flat pitch |

Existing installs keep their saved options. To get these after an update, copy them in from
[Making more voices](#making-more-voices) or use **⋮ → Reset to defaults** on the
Configuration tab.

## Languages

Enable languages under **Languages**; each adds the 9 built-in voices for that language.

| Code | Language | Voice names | Home Assistant language | Status |
|---|---|---|---|---|
| `us` | English (US) | `paul`, `betty`, ... | en-US | Best |
| `uk` | English (UK) | `paul_uk`, ... | en-GB | Good (British vowels, non-rhotic r) |
| `sp` | Spanish (Spain) | `paul_sp`, ... | es-ES | Good |
| `la` | Spanish (Latin America) | `paul_la`, ... | es-419 | Good |
| `gr` | German | `paul_gr`, ... | de-DE | Fair |
| `fr` | French | `paul_fr`, ... | fr-FR | Experimental |

Home Assistant lists each voice under its language. Pick a voice in your voice assistant or
`tts.speak`, because Home Assistant does not tell the add-on which language you want. Custom
voices take an optional `language` (default `us`), and `[:lang xx]` switches language inside a
message. Numbers, times, temperatures, percentages and money are read in each language.

## Making more voices

A custom voice is a **base** voice plus **settings**, written as `code value` pairs separated
by spaces:

```yaml
custom_voices:
  - name: kitt
    base: paul
    settings: ap 114 pr 111 hs 95 sm 15 ri 80 na 10 as 75 qu 70
    rate: 269
  - name: hal
    base: paul
    settings: ap 84 pr 101 hs 96 sm 35 ri 55 br 8 as 35 qu 35
    rate: 235
  - name: glados
    base: betty
    settings: ap 153 pr 101 hs 82 sm 20 ri 70 br 0 as 60 qu 60 pq 95
    rate: 247
  - name: turret
    base: betty
    settings: ap 214 pr 133 hs 102 sm 55 ri 45 br 0 as 40 qu 50 pq 100
    rate: 141
  - name: jarvis
    base: paul
    settings: ap 109 pr 98 hs 97 sm 30 ri 65 as 65 qu 55
    rate: 252
  - name: narrator
    base: harry
    settings: ap 80 pr 86 hs 107 sm 40 ri 70 br 5 as 70 qu 35
    rate: 190
  - name: robot
    base: paul
    settings: pr 0 ap 90      # flat pitch
  - name: heidi              # a German voice
    base: betty
    language: gr
```

- `name` is what you pick in Home Assistant. Reusing a built-in name (e.g. `paul`) replaces it.
- `rate` (optional) sets this voice's words per minute; `language` (optional) its language.
- Try settings live in the preview tool (`go run ./cmd/preview`, see the repository).

### Voice settings

| Code | Setting | Range | Effect |
|---|---|---|---|
| `ap` | Average pitch | 40-500 Hz | Higher = higher voice |
| `pr` | Pitch range | 0-250 % | `0` = monotone robot, high = sing-song |
| `hs` | Head size | 50-160 % | Bigger = deeper, larger-sounding speaker |
| `sx` | Vocal tract | 0 or 1 | 0 = female (head size 85), 1 = male (100) |
| `br` | Breathiness | 0-72 | Breath noise while the voice sounds; high = whisper |
| `lx` | Lax breathiness | 0-100 % | Breath noise that never stops (a voice that never fully closes) |
| `sm` | Smoothness | 0-100 % | Higher = softer, less buzzy |
| `ri` | Richness | 0-100 % | Higher = brighter, more carrying |
| `nf` | Open phase | 0-95 % | Share of each vocal-fold cycle spent open; 0 = automatic |
| `la` | Laryngealization | 0-100 % | Creaky, gravelly voice |
| `na` | Nasality | 0-100 % | Nasal twang; above ~10 it costs clarity |
| `pq` | Pitch steps | 0-100 % | Snaps pitch to whole tones: autotuned, GLaDOS-style |
| `as` | Assertiveness | 0-100 % | How far statements fall at the end |
| `qu` | Quickness | 0-100 % | How fast pitch moves |
| `bf` | Baseline fall | Hz | How far pitch drifts down across a phrase; 0 = automatic |
| `hr` | Hat rise | Hz | Extra rise on a phrase's first accent; 0 = automatic |
| `sr` | Stress rise | Hz | Rise on each accented syllable; 0 = automatic |
| `gv` `gh` `gf` `gn` | Source levels | -30 to 20 dB | Voicing, aspiration (h), frication (s, f), nasal murmur (m, n) |
| `f4` `b4` `f5` `b5` | Upper formants | Hz | 4th and 5th resonance and bandwidth; 0 = automatic |
| `lo` | Loudness | -30 to 12 dB | Output level |

Values outside a range are clamped. Very large head sizes or pitches make speech harder to
understand. The codes follow DECtalk's `[:dv]` names where they exist.

## What it reads well

Retrotalk expands what Home Assistant usually sends: numbers (`1,250`), decimals, negatives,
ordinals (`21st`), years (`1984`), times (`7:30 pm`, `19:00`), temperatures (`72°F`),
percentages, money (`$12.50`), short initialisms spelled out (`TV`, `USB`), common
abbreviations (`Dr.`, `mph`, `kWh`) and accented letters. English pronunciation comes from the
CMU Pronouncing Dictionary (about 126,000 words); unknown words and other languages use
letter-to-sound rules.

Put `*asterisks*` around a word to emphasize it: "It is *really* hot."

## Pronunciations

Fix names and words the engine gets wrong under **Pronunciations**:

```yaml
pronunciations:
  - word: Nguyen
    say: new win            # a respelling in ordinary words
  - word: Hisense
    say: "[hh ay1 s eh2 n s]" # or phonemes in brackets (see below)
```

## Inline commands

Messages can carry DECtalk-style commands in square brackets. Several can share one bracket:
`[:np :ra 200]`.

| Command | Does |
|---|---|
| `[:rate 200]` `[:ra 200]` | Words per minute (75-600) |
| `[:volume 70]` `[:vo 70]` | Volume 0-100 (50 = normal) |
| `[:name hal]` | Switch voice (any built-in or custom voice) |
| `[:np]` `[:nb]` `[:nh]` `[:nf]` `[:nd]` `[:nk]` `[:nu]` `[:nr]` `[:nw]` | Switch to paul, betty, harry, frank, dennis, kit, ursula, rita, wendy |
| `[:dv ap 90 pr 0]` | Change voice settings mid-message |
| `[:lang sp]` | Switch language (`us` `uk` `sp` `la` `gr` `fr`) |
| `[:pause 500]` | Silence, in ms |
| `[:cp 300]` `[:pp 800]` | Pause after commas / sentences, in ms |
| `[:mode spell on]` ... `[:mode spell off]` | Spell letters and digits one by one |
| `[:punct all]` | Read punctuation aloud (`none` / `some` to stop) |
| `[:tone 880,200]` | A tone: frequency in Hz, length in ms |
| `[:dial 5551234]` | Telephone keypad tones |

Example: `[:tone 880,150][:tone 660,250] Someone is at the front door.`

### Phonemes and singing

Text in brackets that isn't a command is read as **phonemes** if it can be: ARPAbet as in
the CMU dictionary (`[hh ah1 l ow]`) or DECtalk-style codes (`[ax bawt]`). Add `<ms>` for
a duration or `<ms,note>` to **sing** a note (1 = C2 ... 37 = C5); `_` is a rest:

```text
[d ey<600,32> z iy<600,29> d ey<600,25> z iy<600,20>]
```

Bracketed words that aren't phonemes, like `[sic]`, are read as text.

## Announcements: everywhere, by room, or by speaker

Use the **Retrotalk announce** script blueprint from the repository:

[![Import blueprint](https://my.home-assistant.io/badges/blueprint_import.svg)](https://my.home-assistant.io/redirect/blueprint_import/?blueprint_url=https%3A%2F%2Fgithub.com%2FTom-Neverwinter%2Fretrotalk-wyoming%2Fblob%2Fmain%2Fblueprints%2Fscript%2Fretrotalk_announce.yaml)

1. Create a label, e.g. **Announcer** (**Settings → Areas, labels & zones → Labels**).
2. Put that label on **one** media player (or its device) per room: the one that should speak there.
   Other speakers in the room stay quiet.
3. Create a script from the blueprint, choosing your Retrotalk TTS entity and the label.
   Optional settings:
   - **Mute switch:** an `input_boolean` or `switch`; while it's on, announcements are skipped.
   - **Quiet hours:** start and end time (e.g. 22:00 to 07:00) when announcements are skipped.
     Same start and end = off.
   - **Gap between announcements:** pause after one finishes before the next queued one plays.
4. Call the script:

```yaml
# Everywhere (every labelled speaker)
action: script.announce
data:
  message: Dinner is ready.

# Specific rooms (their labelled speaker only), at 40% volume
action: script.announce
data:
  message: Laundry is done.
  areas: [kitchen, living_room]
  volume: 40

# Specific speakers (ignores labels and rooms)
action: script.announce
data:
  message: Testing.
  speakers: [media_player.office_speaker]
  voice: robot

# Safety alert: plays even when muted or in quiet hours
action: script.announce
data:
  message: Water leak detected under the kitchen sink.
  priority: true
  volume: 80
```

| Field | What it does |
|---|---|
| `message` | Text to speak (required) |
| `areas` | Only these rooms' labelled speakers |
| `speakers` | Exact media players; overrides `areas` and the label |
| `voice` | Retrotalk voice; empty = add-on default |
| `volume` | 0-100 for this announcement; the previous volume is restored afterwards |
| `priority` | Ignore the mute switch and quiet hours |

How it behaves:
- Announcements **queue** (up to 10) and never talk over each other: the script waits for
  the speakers to stop playing (up to about 5 s + 1 s per 10 characters), then the gap.
- Volume is restored only on players that report a volume. Players that don't report
  `playing` are restored once the time estimate runs out.
- Skipped announcements (muted or quiet hours) are logged in the script's trace with the reason.
- To ignore repeats from a chatty trigger (e.g. a doorbell), make the **calling automation**
  `mode: single` with a short `delay` at the end.

Home Assistant caches generated speech, so repeating the same message is instant.

## Ready-made automations

These automation blueprints use your **announce script** (above), so its rooms, mute switch,
quiet hours and volume handling apply to all of them. Create the announce script first, then
import the ones you want:

| Blueprint | Import | Triggered by | Says |
|---|---|---|---|
| **Mail arrived** | [![Import](https://my.home-assistant.io/badges/blueprint_import.svg)](https://my.home-assistant.io/redirect/blueprint_import/?blueprint_url=https%3A%2F%2Fgithub.com%2FTom-Neverwinter%2Fretrotalk-wyoming%2Fblob%2Fmain%2Fblueprints%2Fautomation%2Fretrotalk_mail.yaml) | A mailbox contact/tilt/vibration sensor opens | Two-tone chime, then "The mail has arrived." Ignores further openings for 30 minutes. |
| **Appointment reminder** | [![Import](https://my.home-assistant.io/badges/blueprint_import.svg)](https://my.home-assistant.io/redirect/blueprint_import/?blueprint_url=https%3A%2F%2Fgithub.com%2FTom-Neverwinter%2Fretrotalk-wyoming%2Fblob%2Fmain%2Fblueprints%2Fautomation%2Fretrotalk_appointment.yaml) | A calendar event is about to start | "Reminder: Dentist starts in 15 minutes, at Main Street Clinic." Skips all-day events. |
| **Attention announcement** | [![Import](https://my.home-assistant.io/badges/blueprint_import.svg)](https://my.home-assistant.io/redirect/blueprint_import/?blueprint_url=https%3A%2F%2Fgithub.com%2FTom-Neverwinter%2Fretrotalk-wyoming%2Fblob%2Fmain%2Fblueprints%2Fautomation%2Fretrotalk_attention.yaml) | A button (dashboard, remote, voice) | Chime + your message ("Dinner is ready"), optional priority and volume. |
| **Laundry or dishes done** | [![Import](https://my.home-assistant.io/badges/blueprint_import.svg)](https://my.home-assistant.io/redirect/blueprint_import/?blueprint_url=https%3A%2F%2Fgithub.com%2FTom-Neverwinter%2Fretrotalk-wyoming%2Fblob%2Fmain%2Fblueprints%2Fautomation%2Fretrotalk_appliance_done.yaml) | A power plug or vibration sensor goes quiet | "The washing machine has finished." Waits a few quiet minutes so mid-cycle pauses don't count. |

Each takes a voice (try `jarvis` for reminders or `glados` for chores) and optional rooms.
Messages can use any Retrotalk command, like the `[:tone ...]` chimes.

## Streaming

Speech is produced a phrase at a time and sent as soon as each phrase is ready, so playback
starts almost immediately. Retrotalk also accepts **streamed text** (Wyoming
`synthesize-start` / `synthesize-chunk`), which Home Assistant uses while a voice
assistant's LLM is still writing its reply: each finished phrase is spoken right away.
