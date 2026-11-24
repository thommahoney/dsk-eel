//go:build !windows

package game

// loopbackReachesOwnAddresses says whether a socket bound to a loopback
// address can send to this machine's other addresses. Here it can.
const loopbackReachesOwnAddresses = true

// unspecifiedReachesThisMachine says whether what is sent to the
// unspecified address, 0.0.0.0 or ::, arrives on this machine. On Linux it
// does. On macOS it was seen not to (macOS 27: a send from a socket that
// is not connected fails with "no route to host"); whether to refuse the
// address there as well is not decided, and until it is macOS goes with
// Linux.
const unspecifiedReachesThisMachine = true
