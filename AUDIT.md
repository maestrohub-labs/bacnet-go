# Library audit — 2026-05-15 — @shamaan0086

Upstream commit: `4a77093799a3a0e256660c6d66f12b1e918e132a`
(`edgeo-scada/bacnet`, "Flatten package structure and update module path",
2026-02-06).

This is the v0.1.0 pre-release audit. One engineer; ~3 hours of structured
read-through. The library is ~4100 lines of Go split across eight files; the
audit was scoped to the seven concerns enumerated in the fork plan
§ 4.C.2.

## What I read

End-to-end, in the order the plan specified:

| File                          | Lines | Notes                                                                                                                              |
| ----------------------------- | ----: | ---------------------------------------------------------------------------------------------------------------------------------- |
| `internal/transport/udp.go`   |   193 | Single UDP socket, RWMutex on the conn, idempotent `Close()`, IPv4-only.                                                            |
| `protocol.go`                 |   682 | BVLC + NPDU + APDU encode/decode + tag/scalar codecs. Length-checked everywhere.                                                    |
| `types.go`                    |  1057 | Enum tables (object types, properties, services, error codes), `ObjectIdentifier`, `DeviceInfo`, `PropertyValue`, `Address`.        |
| `client.go`                   |  1133 | State machine, receiver goroutine, pending map, `Connect`/`Close`, `WhoIs`, `ReadProperty`, `WriteProperty`, `ReadPropertyMultiple`, COV plumbing. |
| `options.go`                  |   280 | Functional options. `slog` logger injection, BBMD config, timeouts, etc.                                                            |
| `errors.go`                   |   363 | Sentinel errors, `BACnetError` (`class`+`code`), `IsTimeout`/`IsDeviceNotFound`/… helpers. Standard `errors.Is`/`errors.As` shape.  |
| `metrics.go`                  |   356 | atomic int64 counters, mutex-guarded latency histogram, snapshot type.                                                              |

## Findings

### Critical (must fix or document before v0.1.0)

1. **`handleCOVNotification` is a TODO stub** — `client.go:393–396`. The
   subscriber API (`SubscribeCOV`/`UnsubscribeCOV`) sends a valid wire
   subscription, but inbound COV notifications are counted and discarded.
   Registered `COVHandler` callbacks **never fire**.

   *Resolution for v0.1.0:* not implementing COV is out of scope, so we
   document this honestly in the README "Status" matrix and call it out in
   `CHANGELOG.md`. We do **not** silently leave `SubscribeCOV` looking
   functional. (Considered removing the COV API entirely; rejected because
   `SubscribeCOV` *does* send a valid subscription request, which a caller
   might still want as a fire-and-forget signal.)

2. **Send-on-closed-channel race in `Close()` vs in-flight `handlePacket`.**
   `client.go:221` spawns `go c.handlePacket(...)` per received packet.
   `Close()` cancels the receiver and waits for the receiver loop
   (`<-receiverDone`, line 161), but does **not** wait for in-flight
   `handlePacket` goroutines to drain. It then `close(ch)`s every pending
   channel under `pendingMu` (line 167). A `handlePacket` that started before
   `receiverCancel()` can race past `pendingMu.RUnlock()` and reach the
   non-blocking send at line 405–408 *after* `Close` closed `ch`, producing
   `panic: send on closed channel`. Window is small but observable under load.

   *Resolution for v0.1.0:* a defensive `defer recover()` wrap around the
   send in `handleResponse` is the smallest viable fix; a `sync.WaitGroup`
   on `handlePacket` is cleaner. Tracked as a Phase D fix (`D.6`, added
   below).

3. **`decodeReadPropertyResponse` uses a stale `length` after the optional
   array-index tag** — `client.go:738–741`. Line 738 binds the just-decoded
   length to `_`, then line 740 advances `offset` by `headerLen + length`
   where `length` is *still* the value from the property-identifier decode
   on line 730. For non-array properties the path isn't taken and reads
   work; reading an array *element* whose reply echoes the ArrayIndex
   produces an offset error and a subsequent `ErrInvalidResponse`.

   *Resolution for v0.1.0:* one-line fix in Phase D (`D.7`).

4. **`WithRetries` / `WithRetryDelay` are dead options.** Set via functional
   options, persisted on `clientOptions`, but `grep retries client.go`
   returns nothing — no retry loop exists. The README and option docs imply
   resilient behavior that isn't there.

   *Resolution for v0.1.0:* not implementing a retry loop now (it interacts
   with the invoke-ID lifecycle and would expand the audit surface). We
   document the no-op in `CHANGELOG.md` and the README "Caveats" section.
   Future work tracked.

### Non-critical (track for v0.1.1 or v0.2.0)

| # | Location                          | Issue                                                                                                                     |
| - | --------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| 5 | `internal/transport/udp.go`       | IPv4-only (`net.ResolveUDPAddr("udp4", …)`, `net.ListenUDP("udp4", …)`). No BACnet/IPv6 (BVLC6) support.                  |
| 6 | `protocol.go:613–626, 629–646`    | `DecodeUnsigned`/`DecodeSigned` silently return `0` on lengths other than 1/2/3/4 instead of erroring. Permissive.        |
| 7 | `protocol.go:667–673`             | `DecodeCharacterString` discards the character-set byte. Non-UTF-8 devices (charsets 1–4) return mojibake as Go `string`. |
| 8 | `protocol.go:192–212`             | `EncodeConfirmedRequest` always emits non-segmented APDUs. Requests over `MaxAPDULength=1476` will be rejected.            |
| 9 | `client.go:206`                   | Receiver loop reads with a 100 ms timeout to poll its context. Up to 100 ms latency on `Close()`.                          |
| 10 | `client.go:376–390`              | `c.devices` cache grows unboundedly. No TTL, no eviction. Long-running clients accumulate stale `DeviceInfo`.              |
| 11 | `client.go:221`                  | Per-packet goroutine spawn. No bound on concurrent in-flight `handlePacket`s.                                              |
| 12 | `options.go:79–84`               | Default `localAddress=""` makes `net.ListenUDP` pick an ephemeral port. **Will not receive** unicast traffic addressed to BACnet port 47808 (COV notifications, unsolicited I-Am from periodic broadcasts). |
| 13 | `types.go:1043–1057`             | `encodeUint16`/`encodeUint32`/`decodeUint16`/`decodeUint32` are unused dead code (also flagged by `staticcheck`).          |
| 14 | `client.go:155 + 113`            | `Connect`/`Close` state transitions are not symmetrically guarded: `Connect` uses CAS, `Close` does an unconditional `Store`. A racy concurrent `Connect`/`Close` pair can leave the client in an inconsistent state. Realistic risk is low (callers do not normally race these). |

### Known limits / consumer caveats

These are inherent to the BACnet spec or to choices we accept as correct for
v0.1.0. They are documented in `README.md` so consumers are not surprised.

- **8-bit invoke IDs** wrap at 256 in-flight requests; with ≥ 256 concurrent
  requests, IDs collide. This matches the BACnet/IP spec (`client.go:192`).
  Consumers should bound concurrency well below 256.
- **Windows port-bind race.** `internal/transport/udp.go` does not set
  `SO_REUSEADDR`/`SO_REUSEPORT`. Two `Client` instances binding the same
  ephemeral port on Windows can race; consumers that construct multiple
  clients in parallel should serialize `NewClient`/`Connect` calls. The
  guard is intentionally consumer-layer, not library-layer.
- **No public "register device by IP" path.** `ReadProperty(ctx, deviceID,
  …)` resolves the device-ID to a `*net.UDPAddr` via the cache populated by
  `WhoIs` (or auto-falls-back to a targeted `WhoIs` on cache miss —
  `client.go:646–683`). There is no exported helper to register a device
  out-of-band with a known IP/port. We add one in Phase D
  (`NewRemoteDevice` / `RegisterDevice`) — see § Phase D below.
- **BBMD is lazy.** Without `WithBBMD(...)`, no BBMD goroutines or sockets
  are created (`client.go:138`). Good.
- **Timeout is per-RPC.** `sendRequest` waits on the caller's `ctx`
  (`client.go:461–464`), not on a shared client-level timer. Good.
- **Per-file copyright header policy** (from `FORK.md`): existing upstream
  files keep their `// Copyright 2025 Edgeo SCADA` header. Substantially
  modified files (so far: `version.go`) carry both upstream's and ours.
- **APDU receiver malformed-packet behavior is safe.** `DecodeBVLC`,
  `DecodeNPDU`, `DecodeAPDU`, and `DecodeTagNumber` all length-check before
  every byte access. `handlePacket` logs at Debug and returns on any decode
  error rather than wedging the receiver loop. Verified in code.

### Deviations from the fork plan

- **`version.go` was not stripped to a bare `const`.** Upstream exports
  `VersionInfo` and `GetVersion()`; the plan suggested replacing with a
  simple `const Version`. We kept the upstream API and just bumped the
  version constants to `0.1.0`. Reason: minimize public-API divergence per
  the plan's own "match upstream surface" principle. (Plan § 3.B.5)

## Wire-level spot checks

Deferred to Phase E (integration tests). The integration suite will:

- Issue a `ReadProperty(Object_Name)` against `shamaan0086/bacnet-sim:latest`,
  capture the wire packet, and assert the byte sequence matches BACnet/IP
  spec § 20.1.2 (will commit the capture under `testdata/`).
- Same for `ReadPropertyMultiple` (spec § 20.1.4).
- Same for `ReadProperty` with `ArrayIndex=ArrayAll` once Phase D.1 lands.

If a wire mismatch is found at that stage, this audit gets a follow-up
revision and the offending encoder is fixed before tagging v0.1.0.

## Folded into Phase D

The following fixes were promoted out of the audit into Phase D rather than
deferred:

- **D.6** — wrap the send in `handleResponse` (`client.go:404–409`) with
  `recover()` (or a `WaitGroup`) so `Close()` cannot race a `handlePacket`
  into a closed channel.
- **D.7** — fix the `decodeReadPropertyResponse` stale-`length` bug at
  `client.go:740`.

## Conclusion

No finding rises to a "fall back to `ulbios/bacnet` rebuild" level (see
plan § 11.5 / issue #2299 escalation criteria). The critical items are
either honestly documentable (#1, #4) or one-line fixes (#2, #3). v0.1.0
can ship on this fork.
