# DECtalk TTS

> 🚧 Work in progress. See the [repository](https://github.com/Tom-Neverwinter/dectalk-wyoming)
> for status.

## Setup

1. Start the add-on. The first install compiles DECtalk, which takes several minutes on a Pi.
2. Home Assistant should show **Wyoming Protocol** under **Settings → Devices & services →
   Discovered**. Click **Configure**.
   - Not discovered? Add **Wyoming Protocol** manually with the **Hostname** shown on this
     add-on's Info tab and port `10205`.
3. Pick DECtalk in **Settings → Voice assistants**, or use the `tts.speak` action.

## Options

| Option | What it does |
|---|---|
| **Default voice** | Voice used when a request doesn't name one. Any built-in or custom voice name. |
| **Speaking rate** | Words per minute for all voices (75-600). Unset = DECtalk default. |
| **Custom voices** | Your own voices (below). They show up in Home Assistant's voice pickers. |

Restart the add-on after changing options, then **reload** the Wyoming integration so
Home Assistant sees new voices.

## Built-in voices

| Voice | Sounds like |
|---|---|
| `paul` | Perfect Paul: the classic default male voice |
| `harry` | Huge Harry: large, deep male |
| `frank` | Frail Frank: older male |
| `dennis` | Doctor Dennis: softer male |
| `betty` | Beautiful Betty: default female |
| `ursula` | Uppity Ursula: assertive female |
| `rita` | Rough Rita: gravelly female |
| `wendy` | Whispering Wendy: breathy female |
| `kit` | Kit the Kid: child |

## Bundled custom voices

These ship as defaults in **Custom voices**, so you can edit or delete them there.
They are DECtalk *impressions* of famous fictional AI voices, not recordings or clones.

| Voice | Inspired by | How it's tuned |
|---|---|---|
| `kitt` (**default**) | KITT, *Knight Rider* | Crisp and bright, lively pitch, brisk 185 wpm |
| `hal` | HAL 9000, *2001: A Space Odyssey* | Low, calm, smooth, narrow pitch range, unhurried 150 wpm |
| `robot` | Classic monotone robot | Flat pitch |

Existing installs keep their saved options. To get these after an update, copy them in from
the list below or use **⋮ → Reset to defaults** on the Configuration tab.

## Making more voices

A custom voice is a **base** voice plus DECtalk **design-voice** settings, written as
`code value` pairs separated by spaces (lowercase letters, digits and spaces only):

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
    settings: hs 130 ap 60
    rate: 140
  - name: chipmunk
    base: kit
    settings: hs 75 ap 300
    rate: 260
```

- `name` is what you pick in Home Assistant. Reusing a built-in name (e.g. `paul`) replaces it.
- `rate` (optional) overrides the global speaking rate for this voice.

### Design-voice settings

Commonly useful codes. Ranges are approximate; DECtalk clamps out-of-range values.

| Code | Setting | Range | Effect |
|---|---|---|---|
| `ap` | Average pitch | 50-350 Hz | Higher = higher voice |
| `pr` | Pitch range | 0-250 % | `0` = monotone robot, high = sing-song |
| `hs` | Head size | 65-145 % | Bigger = deeper, larger-sounding speaker |
| `sx` | Sex | 0 or 1 | 0 = female, 1 = male vocal tract |
| `br` | Breathiness | 0-72 dB | Adds breath noise |
| `sm` | Smoothness | 0-100 % | Higher = softer, less buzzy |
| `ri` | Richness | 0-100 % | Higher = brighter, more carrying |
| `la` | Laryngealization | 0-100 % | Creaky / gravelly voice |
| `as` | Assertiveness | 0-100 % | How strongly sentences end |
| `qu` | Quickness | 0-100 % | How fast pitch changes |
| `lo` | Loudness | 0-86 dB | Output level |

### Trying settings before saving

DECtalk commands work inside the message, so you can audition a voice from
**Developer tools → Actions** without restarting the add-on:

```yaml
action: tts.speak
target:
  entity_id: tts.dectalk
data:
  media_player_entity_id: media_player.kitchen
  message: "[:dv hs 130 ap 60] Fee fi fo fum."
  options:
    voice: harry
```

Copy the `[:dv ...]` contents into `settings` when you like it. Other inline commands:
`[:rate 250]`, `[:phoneme on]` (for singing), `[:punct all]`.

## Announcements: everywhere, by room, or by speaker

Use the **DECtalk announce** script blueprint from the repository:

[![Import blueprint](https://my.home-assistant.io/badges/blueprint_import.svg)](https://my.home-assistant.io/redirect/blueprint_import/?blueprint_url=https%3A%2F%2Fgithub.com%2FTom-Neverwinter%2Fdectalk-wyoming%2Fblob%2Fmain%2Fblueprints%2Fscript%2Fdectalk_announce.yaml)

1. Create a label, e.g. **Announcer** (**Settings → Areas, labels & zones → Labels**).
2. Put that label on **one** media player (or its device) per room: the one that should speak there.
   Other speakers in the room stay quiet.
3. Create a script from the blueprint, choosing your DECtalk TTS entity and the label.
4. Call the script:

```yaml
# Everywhere (every labelled speaker)
action: script.announce
data:
  message: Dinner is ready.

# Specific rooms (their labelled speaker only)
action: script.announce
data:
  message: Laundry is done.
  areas: [kitchen, living_room]

# Specific speakers (ignores labels and rooms)
action: script.announce
data:
  message: Testing.
  speakers: [media_player.office_speaker]
  voice: robot
```

Home Assistant caches generated speech, so repeating the same message is instant.

## Streaming

Audio is streamed: DECtalk writes raw samples to a pipe and the add-on forwards them as they
are produced, so playback can start before the whole message is synthesized. Nothing is
written to disk.

The add-on also accepts **streamed text** (Wyoming `synthesize-start` / `synthesize-chunk`),
which Home Assistant uses when a voice assistant's LLM is still writing its reply. DECtalk runs
in clause mode, speaking each clause as soon as it is complete.
