# Security policy

If you have found a security issue in `bacnet-go`, please report it
privately to the repository owner via a direct message on GitHub — do
**not** open a public GitHub issue for security bugs.

Acknowledgement is best-effort but typically within five business
days. Critical fixes are prioritized over feature work; non-critical
issues are scheduled against the regular release cadence.

## Scope

This library is a BACnet/IP **client**. It opens UDP sockets, sends
requests to BACnet devices, and parses responses. Reasonable security
concerns include:

- Decoder paths that can be panicked or crashed by a malformed packet
  from a peer. A panic in `handlePacket` propagates to the process
  hosting the client. The v0.1.0 audit explicitly walked every decode
  path for length-checks; one panic-on-boolean bug was found in
  `decodePropertyValue` and fixed (see
  [`AUDIT.md`](AUDIT.md) finding #5).
- Memory exhaustion from a flood of spoofed responses. The library
  spawns one goroutine per received packet (audit finding #11) without
  bounded concurrency; high packet rates from a malicious or
  misconfigured network can amplify into resource pressure.
- BBMD foreign-device registration. The library can register with a
  BBMD using a TTL the caller controls; this expands the device's
  attack surface to the BACnet internetwork. Default off.

## Out of scope

- The BACnet protocol itself lacks authentication/integrity at the
  default service level. We do not implement BACnet/SC. Treat the BACnet
  network as untrusted-but-segregated, not as a security boundary.
- We do not currently scan the dependency tree on a schedule; v0.1.0
  ships with no third-party `require` block, so there is no transitive
  surface to monitor.
