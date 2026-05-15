# Contributing to bacnet-go

Thanks for the interest. This document covers the boring practical
things — how to run the tests, how to bring up the BACnet simulator,
what to put in a PR.

## Local development

```
git clone git@github.com:maestrohub-labs/bacnet-go.git
cd bacnet-go

# Unit tests (no network):
make test

# Integration tests (need the BACnet simulator container running):
make sim-up        # starts shamaan0086/bacnet-sim via the parent's
                   #   docker-compose.yaml (port 47810/udp)
make test-integration
make sim-down

# Full coverage report:
make coverage
```

The integration suite reads the simulator's host/port/device-id from
environment variables, defaulting to `127.0.0.1:47810` and device `1234`
to match the supplied `docker-compose.yaml`. Override with
`BACNET_SIM_HOST`, `BACNET_SIM_PORT`, `BACNET_SIM_DEVICE_ID` if your
sim lives elsewhere.

If the simulator is not reachable, every test in `integration_test.go`
skips (with a clear message) rather than failing. This keeps CI runs
without Docker green.

## Style

- Pure Go, `go 1.23` minimum, no CGo. Hard constraint.
- `go.mod` carries no `require` block (stdlib only); we accept at most
  `golang.org/x/sys` if a platform-specific need ever justifies it.
  Anything else gets rejected at review.
- Every new file we author starts with the `maestrohub-labs` Apache-2.0
  header (template in [`FORK.md`](FORK.md)). Upstream files keep their
  original header; substantially modified files get our line added as
  a second header line, not as a replacement.

## PR checklist

Before requesting review, run locally:

- [ ] `make vet`
- [ ] `make build`
- [ ] `make test` (with `-race`)
- [ ] `make test-integration` (with `-race`, against the simulator)
- [ ] `make coverage` — note the result in the PR description
- [ ] `go-licenses check ./...` if your change touches `go.mod`
- [ ] [`AUDIT.md`](AUDIT.md) updated if you changed structural behavior
  (state machine, receiver lifecycle, decode paths)
- [ ] [`FORK.md`](FORK.md) updated if you introduced a new divergence
  from upstream worth recording

## Release process

Releases are manual. Branch protection is light, the repository has a
single admin, and there is no CI cutting tags automatically. To cut a
release:

1. Update [`CHANGELOG.md`](CHANGELOG.md) and `version.go`.
2. `make release-check` from a clean checkout. Every step has to be
   green. If any fail, do not tag.
3. `git tag -a vX.Y.Z -m "..."`, `git push origin vX.Y.Z`.
4. Draft a GitHub Release from the tag with the new
   `CHANGELOG.md` section pasted into the body.

## Reporting issues

Bugs and feature requests go in the GitHub issue tracker on this
repository. Please include:

- Go version and OS (the v0.1.0 audit flagged a Windows-specific
  port-bind race; OS context matters).
- Whether the issue is unit-test-reproducible or only shows up against
  a real BACnet device.
- For wire-level issues: a `tcpdump` / Wireshark capture is gold.
