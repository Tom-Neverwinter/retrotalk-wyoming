//go:build race

package tts

// raceSlowdown scales time limits in tests: the race detector makes code 10-40x slower.
const raceSlowdown = 40
