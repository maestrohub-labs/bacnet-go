# bacnet-go

[![Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

An Apache-2.0-licensed BACnet/IP client library in pure Go.

This repository is a forked, sanitized, library-only derivative of
[edgeo-scada/bacnet](https://github.com/edgeo-scada/bacnet) — the CLI and its
configuration dependencies have been stripped; only the BACnet/IP client code
remains. See [`FORK.md`](FORK.md) and [`NOTICE`](NOTICE) for attribution and
the base of record.

## Status

This is the **`v0.1.0` initial release**. The scope of "supported" is narrow on
purpose; we ship what we test.

| Service                  | Status                                                                       |
| ------------------------ | ---------------------------------------------------------------------------- |
| `ReadProperty`           | Supported and tested (unit + integration against `shamaan0086/bacnet-sim`). |
| `ReadPropertyMultiple`   | Supported and tested.                                                        |
| `WhoIs` / device discovery | Supported and exercised by the example; tested at integration level.       |
| `WriteProperty`          | Present in upstream; **not covered by our test suite**. Use at your own risk. |
| `SubscribeCOV` / `UnsubscribeCOV` | Present in upstream; **not covered by our test suite**.             |
| BBMD foreign-device registration | Present in upstream; **not covered by our test suite**.             |

See [`AUDIT.md`](AUDIT.md) for the one-engineer read-through audit, known
limits, and consumer caveats.

## Install

```
go get github.com/maestrohub-labs/bacnet-go@v0.1.0
```

Pure Go, no CGo, builds with `CGO_ENABLED=0`. Targets Go 1.23+.

## Minimal usage

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	bacnet "github.com/maestrohub-labs/bacnet-go"
)

func main() {
	client, err := bacnet.NewClient(bacnet.WithTimeout(3 * time.Second))
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	devices, err := client.WhoIs(ctx, bacnet.WithDiscoveryTimeout(5*time.Second))
	if err != nil {
		log.Fatal(err)
	}
	if len(devices) == 0 {
		fmt.Println("no devices discovered")
		return
	}

	target := devices[0].ObjectID.Instance
	name, err := client.ReadProperty(ctx, target,
		bacnet.NewObjectIdentifier(bacnet.ObjectTypeDevice, target),
		bacnet.PropertyObjectName,
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("device %d Object_Name = %v\n", target, name)
}
```

A runnable copy lives at [`examples/read/main.go`](examples/read/main.go).

> **Caveat — device addressing requires Who-Is first.** `ReadProperty` is keyed
> on a numeric device-instance ID; the client resolves that ID to an IP/port
> via its internal device cache, which is populated by `WhoIs`. If you call
> `ReadProperty` against an unknown device ID, the client transparently issues
> a `WhoIs` on the local subnet to discover it. A direct
> "I-already-know-the-IP" registration helper is not yet exposed in v0.1.0;
> see `AUDIT.md` for details.

## Caveats

- **8-bit invoke IDs.** BACnet invoke IDs are 8-bit (per spec). With ≥ 256
  requests in flight, IDs wrap and collide. For our intended low-concurrency
  workloads this is fine; multi-device, high-concurrency callers should be
  aware.
- **Windows port-bind race.** Constructing multiple `Client` instances
  concurrently on Windows can occasionally race on UDP bind. Consumers should
  serialize `NewClient`/`Connect` calls during process startup if they expect
  to create more than one client at a time. The library does not add this
  guard itself (it is consumer-layer policy).
- **No CGo.** Pure Go only. The library builds cleanly with `CGO_ENABLED=0`.
- **Dependency surface.** Module `require` block is empty — stdlib only.

## Running the tests

```
# unit tests, no network
make test

# integration tests against the BACnet simulator container
# (requires Docker; the suite skips cleanly if Docker is unavailable)
make test-integration

# coverage report
make coverage
```

## Project status

Maintained by `maestrohub-labs` for use in the MaestroHub platform's BACnet
connector. PRs are welcome but reviews are best-effort — fixes that affect our
use cases are prioritized.

We do not track upstream on a schedule. See `FORK.md` for the cherry-pick
policy.

## License & attribution

This library is licensed under the [Apache License, Version 2.0](LICENSE).

Original work © 2025 Edgeo SCADA. Modifications © 2026 maestrohub-labs. See
[`NOTICE`](NOTICE) for the full attribution and [`FORK.md`](FORK.md) for the
list of significant changes from upstream.
