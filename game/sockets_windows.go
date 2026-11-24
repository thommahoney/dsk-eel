package game

// loopbackReachesOwnAddresses says whether a socket bound to a loopback
// address can send to this machine's other addresses. On Windows it cannot:
// every send fails with "unreachable network".
const loopbackReachesOwnAddresses = false

// unspecifiedReachesThisMachine says whether what is sent to the
// unspecified address, 0.0.0.0 or ::, arrives on this machine. On Windows
// it does not: every send to it fails.
const unspecifiedReachesThisMachine = false
