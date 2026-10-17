# Retrotalk TTS

> 🚧 Work in progress. See the [repository](https://github.com/Tom-Neverwinter/retrotalk-wyoming)
> for status.

Retrotalk is a local, retro-sounding text-to-speech engine in the style of 1980s formant
synthesizers. It runs entirely on your Home Assistant machine, with no cloud and no internet.
English only for now.

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
| **Custom voices** | Your own voices (below). They show up in Home Assistant's voice pickers. |

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

## Bundled custom voices

These ship as defaults in **Custom voices**, so you can edit or delete them there.
They are *impressions* of famous fictional AI voices, not recordings or clones.

| Voice | Inspired by | How it's tuned |
|---|---|---|
| `kitt` (**default**) | KITT, *Knight Rider* | Crisp and bright, lively pitch, brisk 185 wpm |
| `hal` | HAL 9000, *2001: A Space Odyssey* | Low, calm, smooth, narrow pitch range, unhurried 150 wpm |
| `robot` | Classic monotone robot | Flat pitch |

Existing installs keep their saved options. To get these after an update, copy them in from
the list below or use **⋮ → Reset to defaults** on the Configuration tab.

## Making more voices

A custom voice is a **base** voice plus **settings**, written as `code value` pairs separated
by spaces:

```yaml
custom_voices:
  - name: kitt
    base: paul
    settings: ap 125 pr 120 ri 70 hs 95 as 70
    rate: 185
  - name: hal
    base: paul
    settings: ap 95 pr 35 sm 70 hs 105 as 30
    rate: 150
  - name: robot
    base: paul
    settings: pr 0 ap 90      # flat pitch
  - name: giant
    base: harry
    settings: hs 125 ap 70
    rate: 140
  - name: chipmunk
    base: kit
    settings: hs 72 ap 320
    rate: 240
```

- `name` is what you pick in Home Assistant. Reusing a built-in name (e.g. `paul`) replaces it.
- `rate` (optional) sets this voice's words per minute.

### Voice settings

| Code | Setting | Range | Effect |
|---|---|---|---|
| `ap` | Average pitch | 40-500 Hz | Higher = higher voice |
| `pr` | Pitch range | 0-250 % | `0` = monotone robot, high = sing-song |
| `hs` | Head size | 50-160 % | Bigger = deeper, larger-sounding speaker |
| `sx` | Vocal tract | 0 or 1 | 0 = female (head size 85), 1 = male (100) |
| `br` | Breathiness | 0-72 | Adds breath noise; high = whisper |
| `sm` | Smoothness | 0-100 % | Higher = softer, less buzzy |
| `ri` | Richness | 0-100 % | Higher = brighter, more carrying |
| `la` | Laryngealization | 0-100 % | Creaky, gravelly voice |
| `as` | Assertiveness | 0-100 % | How far statements fall at the end |
| `qu` | Quickness | 0-100 % | How fast pitch moves |
| `lo` | Loudness | -30 to 12 dB | Output level |

Values outside a range are clamped. Very large head sizes or pitches make speech harder to
understand.

## What it reads well

Retrotalk expands what Home Assistant usually sends: numbers (`1,250`), decimals, negatives,
ordinals (`21st`), years (`1984`), times (`7:30 pm`, `19:00`), temperatures (`72°F`),
percentages, money (`$12.50`), short initialisms spelled out (`TV`, `USB`), common
abbreviations (`Dr.`, `mph`, `kWh`) and accented letters. Pronunciation comes from the CMU
Pronouncing Dictionary (about 126,000 words); unknown words fall back to letter-to-sound rules.

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

## Streaming

Speech is produced a phrase at a time and sent as soon as each phrase is ready, so playback
starts almost immediately. Retrotalk also accepts **streamed text** (Wyoming
`synthesize-start` / `synthesize-chunk`), which Home Assistant uses while a voice
assistant's LLM is still writing its reply: each finished phrase is spoken right away.
