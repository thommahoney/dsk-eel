//go:build race

package game

// raceDetector says whether the tests were built with -race. The detector
// makes a tick about thirty times slower, so the long random-play tests run
// fewer ticks under it.
const raceDetector = true
