//go:build race

package controller

// raceDetector says whether the tests were built with -race. The detector
// makes the key reader many times slower, so the test that types every
// short string of bytes at it types shorter ones under it.
const raceDetector = true
