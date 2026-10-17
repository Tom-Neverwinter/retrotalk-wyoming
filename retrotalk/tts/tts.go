// Package tts is a small English text-to-speech engine: text normalization, CMUdict
// pronunciation with letter-to-sound fallback, Klatt-style duration and pitch rules, and a
// Klatt cascade/parallel formant synthesizer. Output is 16-bit mono PCM at SampleRate.
package tts

// Speak synthesizes text, calling emit with little-endian 16-bit PCM for each phrase as
// soon as it is ready. It stops at the first emit error.
func Speak(text string, v Voice, emit func(pcm []byte) error) error {
	for _, ph := range phrases(normalize(text)) {
		segs := segments(ph)
		if len(segs) == 0 {
			continue
		}
		timeAndPitch(segs, ph.end, v)
		if err := emit(newSynth(v).render(frames(segs, ph.end, v))); err != nil {
			return err
		}
	}
	return nil
}
