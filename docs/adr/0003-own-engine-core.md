# ADR-0003: Replace the goed2k runtime core with our own engine

## Status

Accepted

## Decision

The vendored `goed2k` stays only until our own engine reaches parity behind
the Sidecar's NDJSON contract; then `third_party/goed2k` is deleted.

The new engine keeps the proven parts — wire codecs, ed2k hashing, server.met /
nodes.dat / link parsing, Kad routing and traversal algorithms — behind
narrower interfaces, and rewrites the runtime core: session, transfer, peer
session, connection, upload queue, and storage.

One goroutine owns all engine state. Each connection has a reader goroutine
that delivers decoded frames to the owner; commands are posted as functions;
snapshots are immutable values built by the owner. A peer session is a state
machine from received packets to actions. Rate Limits are token buckets on the
connection write and read paths.

The engine reads the existing `state.json`, so identities, credits, and
unfinished transfers survive the switch.

## Considered Options

- Refactor `goed2k` in place: its mutex does not guard the tick thread's
  mutations (`go test -race` reports data races), and Session and
  PeerConnection couple every subsystem, so no piece can move alone.
- Rewrite everything: discards codecs and Kad interoperability fixes that
  already have test vectors.
- Goroutine per peer with shared locks: keeps the race surface we are leaving.

## Consequences

- Sidecar tests run against both engines until the switch.
- Engine tests use a fake clock and in-memory connections; tests that reach the
  real network run only when explicitly enabled.
- First version adds Rate Limits, a configurable connection limit, automatic
  Kad publishing after completion, real source exchange answers, and banning
  peers that send corrupt blocks. LowID callbacks, obfuscation, UDP reask, and
  AICH come later; obfuscation waits for a measurement of ISP throttling.
- Code may be reused from `goed2k` (MIT) with attribution; eMule and aMule
  (GPL) contribute protocol knowledge only.
