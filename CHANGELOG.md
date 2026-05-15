# Changelog

All notable changes to `maestrohub-labs/bacnet-go` are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versioning is semantic.

## [0.1.0] — 2026-05-15

Initial release.

Forked from
[edgeo-scada/bacnet@4a77093](https://github.com/edgeo-scada/bacnet/commit/4a77093799a3a0e256660c6d66f12b1e918e132a).
See [`FORK.md`](FORK.md) for the base of record and the per-file
copyright-header policy; see [`AUDIT.md`](AUDIT.md) for the one-engineer
read-through audit and the known-limits / consumer-caveats list.

### Tested and supported

- `ReadProperty` (BACnet/IP, confirmed-service, application-tag-aware decode).
- `ReadPropertyMultiple` (one round-trip, multiple properties).
- `WhoIs` device discovery and the `Who-Is → I-Am → device cache` flow.
- Out-of-band device addressing via `NewRemoteDevice` + `RegisterDevice`
  (skips Who-Is when caller has IP/port/instance up front).

### Present but not covered by the test suite

- `WriteProperty`, `SubscribeCOV`, `UnsubscribeCOV`, BBMD foreign-device
  registration. `SubscribeCOV` is **broken upstream**: the subscription
  request is sent on the wire, but `handleCOVNotification` is a TODO
  stub — registered handlers never fire. Documented in `README.md` and
  `AUDIT.md` finding #1.

### Changed (vs. upstream)

- Removed the `cmd/edgeo-bacnet` CLI, `examples/basic`, `go.work`, and the
  CLI-oriented `Makefile`.
- Trimmed the dependency tree from ~20 modules (cobra/viper/uber-atomic/…)
  to **stdlib only**. `go.mod` carries no `require` block.
- Rewrote the module path from `github.com/edgeo-scada/bacnet` to
  `github.com/maestrohub-labs/bacnet-go`.
- Rewrote `README.md` to be library-focused, honest about the supported
  surface, and to call out the COV / retries no-op caveats explicitly.
  (Upstream README incorrectly claimed MIT licensing; we say Apache-2.0,
  which matches the actual `LICENSE` and per-file headers.)
- Bumped `version.go` to `0.1.0` and added a maestrohub-labs copyright
  line alongside the upstream Edgeo SCADA notice (`FORK.md` policy).
  `VersionInfo` and `GetVersion()` are preserved as part of the public
  API rather than collapsed to a bare constant — minimizes divergence
  from upstream's surface.

### Added

- `ArrayAll uint32 = 0xFFFFFFFF` constant in `types.go` for the
  BACnet spec-defined "read whole array" sentinel.
- `NewRemoteDevice(deviceID, ip, port) DeviceInfo` constructor and
  `(*Client).RegisterDevice(DeviceInfo)` method in `device.go`. Lets
  callers populate the client's device cache out of band with a known
  IP — no `WhoIs` round-trip required. Documented in the README's
  "Out-of-band addressing" section.
- `Makefile` with library-friendly targets: `test`, `test-integration`,
  `coverage`, `sim-up`/`sim-down`/`sim-logs` for managing the local
  BACnet simulator container, `license-scan`, and a one-shot
  `release-check` that walks the pre-release checklist.
- Unit + integration test suite:
  - **Unit (no network):** state machine, invoke-ID wrap, `Close`
    idempotency, `Close` unblocks pending requests, `Close` waits for
    in-flight `handlePacket` goroutines, BVLC/NPDU/APDU encode-decode,
    tag short/extended forms, signed/unsigned/real/double encode round-
    trips, malformed-APDU decoding, ObjectIdentifier round-trip,
    `ArrayAll` wire encoding, `NewRemoteDevice` 4-byte/6-byte/IPv6
    paths, `RegisterDevice` cache insertion, BACnet error helpers,
    `decodeError`, `decodeReadPropertyResponse` with and without
    ArrayIndex, `decodePropertyValue` scalar cases, UDP transport
    loopback, transport close-during-Read.
  - **Integration (`-tags=integration`, against the local
    `shamaan0086/bacnet-sim` container):** ConnectReadDisconnect,
    ReadPropertyMultiple, Object_List length, Reconnect-after-disconnect
    (3 cycles), ping latency (< 100 ms on loopback).
- `NOTICE`, `FORK.md`, `AUDIT.md`, this `CHANGELOG.md`, `CONTRIBUTING.md`,
  and `SECURITY.md`.

### Fixed (audit-surfaced upstream bugs)

- **AUDIT.md #2 — send-on-closed-channel race in `Close()`.** The
  receiver loop spawned `handlePacket` goroutines unawaited; `Close`
  closed pending channels before in-flight handlers drained, so a
  late `handleResponse` could panic with "send on closed channel".
  Fixed by tracking `handlePacket` with a `sync.WaitGroup` that
  `Close` waits on after the receiver-loop exit and before closing
  pending channels. Regression covered by
  `TestCloseWaitsForHandlePacket`.
- **AUDIT.md #3 — `decodeReadPropertyResponse` stale-length bug.**
  After decoding the optional `[2]` ArrayIndex tag, the function
  advanced its byte offset using a `length` variable held over from
  the previous iteration (the property-id length). Reads of array
  elements whose reply echoed the ArrayIndex back returned
  `ErrInvalidResponse`. Fixed; regression covered by
  `TestDecodeReadPropertyResponseWithArrayIndex`.
- **AUDIT.md #5 — `decodePropertyValue` panicked on boolean reads.**
  The function sliced `data[headerLen:headerLen+length]` unconditionally
  before its application-tag switch. For `TagBoolean` and `TagNull`
  the value is encoded in the length nibble with no payload byte, so
  the slice over-ran the input and panicked the receiver goroutine.
  Fixed: `TagNull` and `TagBoolean` now short-circuit before the
  slice, and the slice is bounds-checked for the remaining tag types.
  Regression covered by `TestDecodePropertyValueScalars/boolean_true`.

### Known limits (still in v0.1.0)

These are documented in `AUDIT.md` and the `README.md` "Caveats" section.

- **`WithRetries` / `WithRetryDelay` are no-ops.** Documented; no retry
  loop exists in `sendRequest`. Future work.
- **8-bit invoke IDs** wrap at 256 in-flight requests (per BACnet spec).
- **Windows port-bind race.** No `SO_REUSEADDR`; consumers constructing
  multiple clients in parallel should serialize.
- **Default `LocalAddress` binds to an ephemeral port,** not BACnet/IP
  port 47808. Client-initiated reads work; unsolicited inbound traffic
  to port 47808 won't be received unless the caller passes
  `WithLocalAddress(":47808")` explicitly.
- **IPv4-only transport.** No BACnet/IPv6 (BVLC6).
- **`decodeUnsigned`/`decodeSigned` are permissive** — unknown lengths
  return 0 silently. Should be fixed in a follow-up.

### Coverage at the v0.1.0 gate

Per-file statement coverage (unit + integration tests, `-race`):

| File                          | Coverage  | Gate (≥60%) |
| ----------------------------- | --------: | :---------: |
| `protocol.go`                 |   67.5%   |     ✅      |
| `types.go`                    |   81.5%   |     ✅      |
| `internal/transport/udp.go`   |   69.3%   |     ✅      |
| `client.go`                   |   44.1%   |     ❌      |

`client.go` is below the gate because the remaining uncovered statements
are largely the network paths the v0.1.0 test scope does not exercise:
`WriteProperty`, `SubscribeCOV`, BBMD register/forward, the
`handlePacket` error branches for non-Confirmed-Ack PDUs, and the slog
log lines on the unhappy paths. Tracked as future-work in `AUDIT.md`.

### Audit

One-engineer read-through audit committed at
[`AUDIT.md`](AUDIT.md). No findings rose to the "fall back to a
different upstream" escalation threshold per fork plan § 11.5.

[0.1.0]: https://github.com/maestrohub-labs/bacnet-go/releases/tag/v0.1.0
