//go:build !race

package controller

// raceDetector says whether the tests were built with -race.
const raceDetector = false
