//go:build !windows

package game

// loopbackReachesOwnAddresses says whether a socket bound to a loopback
// address can send to this machine's other addresses. Here it can.
const loopbackReachesOwnAddresses = true
