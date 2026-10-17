"""CI smoke test: drive the built image with the wyoming client Home Assistant uses.

  smoke_test.py options     print add-on options (config.yaml defaults, every language) as JSON
  smoke_test.py run OUTDIR  test every voice, streaming and a >4 KB message; save WAVs to OUTDIR
"""
import asyncio
import json
import sys
import time
import wave
from pathlib import Path

import yaml
from wyoming.audio import AudioChunk, AudioStart, AudioStop
from wyoming.client import AsyncTcpClient
from wyoming.error import Error
from wyoming.info import Describe, Info
from wyoming.tts import (Synthesize, SynthesizeChunk, SynthesizeStart, SynthesizeStop,
                         SynthesizeStopped, SynthesizeVoice)

CONFIG = Path(__file__).parent.parent / "retrotalk" / "config.yaml"
HOST, PORT = "127.0.0.1", 10205
TEXT = {
    "en": "Hello from Home Assistant. It is 72°F outside and the front door is open.",
    "es": "Hola. Hace 21 grados y la puerta principal está abierta.",
    "de": "Hallo. Es sind 21 Grad und die Haustür ist offen.",
    "fr": "Bonjour. Il fait 21 degrés et la porte d'entrée est ouverte.",
}
CUSTOM = {
    "kitt": "Good evening, Michael. Scanning the area now.",
    "hal": "I'm sorry, Dave. I'm afraid I can't do that.",
    "robot": "I am a robot. Beep boop.",
}


async def collect(client, done):
    """Read until done(event type); return (rate, pcm, seconds to first audio)."""
    start, first, rate, pcm = time.monotonic(), None, None, bytearray()
    while True:
        event = await asyncio.wait_for(client.read_event(), timeout=120)
        if event is None:
            raise SystemExit("connection closed early")
        if Error.is_type(event.type):
            raise SystemExit(f"server error: {Error.from_event(event).text}")
        if AudioStart.is_type(event.type):
            rate = AudioStart.from_event(event).rate
        if AudioChunk.is_type(event.type):
            first = first or time.monotonic() - start
            pcm += AudioChunk.from_event(event).audio
        if done(event.type):
            return rate, bytes(pcm), first


def save(out, name, rate, pcm, min_seconds=0.5):
    seconds = len(pcm) / (rate * 2)
    with wave.open(str(out / f"{name}.wav"), "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(rate)
        w.writeframes(pcm)
    print(f"  {name:12} {seconds:6.2f}s")
    assert seconds >= min_seconds, f"{name}: only {seconds:.2f}s of audio"
    return seconds


async def speak(text, voice):
    async with AsyncTcpClient(HOST, PORT) as c:
        await c.write_event(Synthesize(text=text, voice=SynthesizeVoice(name=voice)).event())
        return await collect(c, AudioStop.is_type)


async def run(out):
    out.mkdir(parents=True, exist_ok=True)
    for _ in range(60):  # wait for the server
        try:
            async with AsyncTcpClient(HOST, PORT) as c:
                await c.write_event(Describe().event())
                info = Info.from_event(await c.read_event())
            break
        except OSError:
            await asyncio.sleep(1)
    else:
        raise SystemExit("server never came up")

    program = info.tts[0]
    assert program.supports_synthesize_streaming, "streaming not advertised"
    print(f"{len(program.voices)} voices")

    print("every voice:")
    for v in program.voices:
        text = CUSTOM.get(v.name) or TEXT[v.languages[0][:2]]
        rate, pcm, _ = await speak(text, v.name)
        assert rate == 16000, f"{v.name}: rate {rate}"
        save(out, v.name, rate, pcm)

    print("inline commands, singing, spelling:")
    for name, text in (("tone_switch", "[:tone 880,150] Ding. [:name hal] Good afternoon."),
                       ("singing", "[d ey<600,32> z iy<600,29> d ey<600,25> z iy<600,20>]"),
                       ("spell", "[:mode spell on]ABC 123")):
        rate, pcm, _ = await speak(text, "paul")
        save(out, f"cmd_{name}", rate, pcm)

    print("streaming text in (Home Assistant's event order):")
    async with AsyncTcpClient(HOST, PORT) as c:
        voice = SynthesizeVoice(name="kitt")
        await c.write_event(SynthesizeStart(voice=voice).event())
        parts = ["Michael, ", "I am detecting ", "a vehicle ahead. ", "Shall I take ", "the wheel?"]
        for part in parts:
            await c.write_event(SynthesizeChunk(text=part).event())
        await c.write_event(Synthesize(text="".join(parts), voice=voice).event())
        await c.write_event(SynthesizeStop().event())
        rate, pcm, _ = await collect(c, SynthesizeStopped.is_type)
        save(out, "stream_kitt", rate, pcm)

    print("long message and time to first audio (phrase-by-phrase streaming):")
    text = "This is a long announcement used to test streaming. " * 90
    t0 = time.monotonic()
    rate, pcm, first = await speak(text, "paul")
    total = time.monotonic() - t0
    seconds = save(out, "long_paul", rate, pcm, min_seconds=60)
    print(f"  {len(text)} chars: first audio after {first:.2f}s, all {seconds:.0f}s of audio in {total:.2f}s")


if __name__ == "__main__":
    if sys.argv[1] == "options":
        options = yaml.safe_load(CONFIG.read_text())["options"]
        options["languages"] = ["us", "uk", "sp", "la", "gr", "fr"]
        print(json.dumps(options))
    else:
        asyncio.run(run(Path(sys.argv[2])))
