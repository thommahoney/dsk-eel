package game

// loopbackReachesOwnAddresses says whether a socket bound to a loopback
// address can send to this machine's other addresses. On Windows it cannot:
// every send fails with "unreachable network".
const loopbackReachesOwnAddresses = false
