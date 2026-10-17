// Package tts is a small English text-to-speech engine: text normalization, CMUdict
// pronunciation with letter-to-sound fallback, Klatt-style duration and pitch rules, and a
// Klatt cascade/parallel formant synthesizer. Output is 16-bit mono PCM at SampleRate.
package tts

// Speak synthesizes text with voice v (see Speaker for inline commands), calling emit with
// little-endian 16-bit PCM for each phrase as soon as it is ready.
func Speak(text string, v Voice, emit func(pcm []byte) error) error {
	return (&Speaker{}).Speak(text, v, emit)
}
