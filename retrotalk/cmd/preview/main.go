// Command preview serves a local web page for listening to the engine: type text, pick a
// voice, adjust settings, play.
//
//	go run ./cmd/preview [-addr localhost:8099] [-tune constants.json]
package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Tom-Neverwinter/retrotalk-wyoming/tts"
)

func main() {
	addr := flag.String("addr", "localhost:8099", "listen address (keep it on localhost)")
	tuneFile := flag.String("tune", "", "JSON file of tune constant overrides, e.g. an optimizer result")
	flag.Parse()
	label := "shipped constants"
	if *tuneFile != "" {
		b, err := os.ReadFile(*tuneFile)
		if err != nil {
			log.Fatal(err)
		}
		var over map[string]float64
		if err := json.Unmarshal(b, &over); err != nil {
			log.Fatal(err)
		}
		if err := tts.Override(over); err != nil {
			log.Fatal(err)
		}
		label = "constants from " + *tuneFile
	}
	tts.LoadDictionary()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, page, label)
	})
	http.HandleFunc("/speak", speak)
	log.Printf("Retrotalk preview (%s): open http://%s", label, *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

// speak returns the text as a WAV file.
func speak(w http.ResponseWriter, r *http.Request) {
	v, ok := tts.Presets[r.FormValue("base")]
	if !ok {
		http.Error(w, "unknown voice", http.StatusBadRequest)
		return
	}
	v, err := v.With(r.FormValue("settings"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if rate, err := strconv.ParseFloat(r.FormValue("rate"), 64); err == nil && rate >= 75 && rate <= 600 {
		v.Rate = rate
	}
	v.Lang = tts.LanguageTag(r.FormValue("lang"))
	start := time.Now()
	var pcm []byte
	tts.Speak(r.FormValue("text"), v, func(b []byte) error { pcm = append(pcm, b...); return nil })
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("X-Synthesis-Ms", strconv.FormatInt(time.Since(start).Milliseconds(), 10))
	h := struct {
		Riff     [4]byte
		Size     uint32
		Wave     [4]byte
		Fmt      [4]byte
		FmtSize  uint32
		Format   uint16
		Channels uint16
		Rate     uint32
		ByteRate uint32
		Align    uint16
		Bits     uint16
		Data     [4]byte
		DataSize uint32
	}{[4]byte{'R', 'I', 'F', 'F'}, uint32(36 + len(pcm)), [4]byte{'W', 'A', 'V', 'E'},
		[4]byte{'f', 'm', 't', ' '}, 16, 1, 1, tts.SampleRate, tts.SampleRate * 2, 2, 16,
		[4]byte{'d', 'a', 't', 'a'}, uint32(len(pcm))}
	binary.Write(w, binary.LittleEndian, h)
	w.Write(pcm)
}

const page = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Retrotalk preview</title>
<style>
 :root { --bg:#f6f4ef; --fg:#1b1f2a; --muted:#666; --accent:#c77d00; --panel:#fff; }
 @media (prefers-color-scheme: dark) { :root { --bg:#14171f; --fg:#e8e6e1; --muted:#9a9a9a; --accent:#ffb020; --panel:#1b1f2a; } }
 body { margin:0; background:var(--bg); color:var(--fg); font:16px/1.5 system-ui,sans-serif; }
 main { max-width:720px; margin:0 auto; padding:24px 16px; }
 h1 { font-size:1.4rem; margin:0 0 4px } .note { color:var(--muted); font-size:.85rem; margin:0 0 16px }
 textarea, select, input { width:100%%; box-sizing:border-box; font:inherit; padding:8px; border-radius:6px;
   border:1px solid var(--muted); background:var(--panel); color:var(--fg) }
 textarea { min-height:110px } label { display:block; margin:12px 0 4px; font-weight:600 }
 .row { display:grid; grid-template-columns:1fr 1fr; gap:12px }
 button { margin-top:16px; padding:10px 20px; font:inherit; font-weight:600; border:0; border-radius:6px;
   background:var(--accent); color:#000; cursor:pointer } button:disabled { opacity:.5 }
 audio { width:100%%; margin-top:16px } #status { color:var(--muted); font-size:.85rem; margin-top:6px }
 .samples button { margin:4px 4px 0 0; padding:4px 10px; font-weight:400; background:var(--panel); color:var(--fg); border:1px solid var(--muted) }
</style></head>
<body><main>
<h1>Retrotalk preview</h1>
<p class="note">Engine: %s. Runs locally; nothing leaves this PC.</p>
<label for="text">Text</label>
<textarea id="text">The front door has been open for 10 minutes. It is 72°F outside.</textarea>
<div class="samples" id="samples"></div>
<div class="row">
 <div><label for="voice">Voice</label><select id="voice"></select></div>
 <div><label for="lang">Language</label><select id="lang">
  <option value="us">English (US)</option><option value="uk">English (UK)</option>
  <option value="sp">Spanish (Spain)</option><option value="la">Spanish (Latin America)</option>
  <option value="gr">German</option><option value="fr">French</option></select></div>
</div>
<label for="rate">Rate: <span id="rateval"></span> wpm</label><input id="rate" type="range" min="75" max="400" step="5">
<label for="settings">Voice settings <span class="note">(ap pr hs sx br sm ri la as qu lo na pq bf hr sr gv gh gf gn f4 b4 f5 b5 lx nf, e.g. "ap 90 pr 0")</span></label>
<input id="settings">
<button id="go">Speak</button>
<audio id="audio" controls></audio>
<div id="status"></div>
</main>
<script>
const voices = {
  kitt:     {base:"paul", settings:"ap 114 pr 111 hs 95 sm 15 ri 80 na 10 as 75 qu 70", rate:269},
  hal:      {base:"paul", settings:"ap 84 pr 101 hs 96 sm 35 ri 55 br 8 as 35 qu 35", rate:235},
  glados:   {base:"betty", settings:"ap 153 pr 101 hs 82 sm 20 ri 70 br 0 as 60 qu 60 pq 95", rate:247},
  turret:   {base:"betty", settings:"ap 214 pr 133 hs 102 sm 55 ri 45 br 0 as 40 qu 50 pq 100", rate:141},
  jarvis:   {base:"paul", settings:"ap 109 pr 98 hs 97 sm 30 ri 65 as 65 qu 55", rate:252},
  narrator: {base:"harry", settings:"ap 80 pr 86 hs 107 sm 40 ri 70 br 5 as 70 qu 35", rate:190},
  robot:    {base:"paul", settings:"pr 0 ap 90", rate:180},
};
const builtinRates = {paul:180, harry:170, frank:165, dennis:175, betty:180, ursula:185, rita:175, wendy:170, kit:175};
for (const [n, r] of Object.entries(builtinRates)) voices[n] = {base:n, settings:"", rate:r};
const samples = ["Good morning. It is 7:30 am and 68 degrees.", "Motion detected in the garage.",
  "Water leak detected under the kitchen sink.", "I'm sorry, Dave. I'm afraid I can't do that.",
  "The cake is a lie.", "Your package has been delivered.",
  "[:tone 880,150][:tone 660,250] Someone is at the front door.",
  "Calling home. [:dial 5551234]",
  "[d ey<600,32> z iy<600,29> d ey<600,25> z iy<600,20>]",
  "It is *really* important.", "[:mode spell on]ABC 123[:mode spell off] spelled out.",
  "[:name hal]Good afternoon. [:name glados]Hello, and again, welcome.",
  "sp:La puerta principal está abierta.", "gr:Die Haustür ist offen.", "fr:La porte d'entrée est ouverte."];
const $ = id => document.getElementById(id);
for (const n of Object.keys(voices)) $("voice").add(new Option(n, n));
for (const raw of samples) { // "sp:..." samples also pick their language
  const m = raw.match(/^(sp|gr|fr):(.*)$/), s = m ? m[2] : raw;
  const b = document.createElement("button"); b.textContent = s.slice(0, 28) + "…";
  b.onclick = () => { $("text").value = s; $("lang").value = m ? m[1] : "us"; speak(); }; $("samples").append(b); }
function pick() { const v = voices[$("voice").value]; $("settings").value = v.settings; $("rate").value = v.rate; $("rateval").textContent = v.rate; }
$("voice").onchange = pick; $("rate").oninput = () => $("rateval").textContent = $("rate").value;
$("voice").value = "kitt"; pick();
async function speak() {
  $("go").disabled = true; $("status").textContent = "Synthesizing…";
  const body = new URLSearchParams({text:$("text").value, base:voices[$("voice").value].base,
    settings:$("settings").value, rate:$("rate").value, lang:$("lang").value});
  const res = await fetch("/speak", {method:"POST", body});
  $("go").disabled = false;
  if (!res.ok) { $("status").textContent = "Error: " + await res.text(); return; }
  const blob = await res.blob();
  $("audio").src = URL.createObjectURL(blob); $("audio").play();
  $("status").textContent = "Synthesized in " + res.headers.get("X-Synthesis-Ms") + " ms, " + (blob.size / 32000).toFixed(1) + " s of audio.";
}
$("go").onclick = speak;
$("text").onkeydown = e => { if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) speak(); };
</script>
</body></html>
`
