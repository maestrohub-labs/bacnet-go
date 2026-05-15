# Fork record

This repository is a fork of [edgeo-scada/bacnet](https://github.com/edgeo-scada/bacnet).

## Base of record

- **Upstream URL:** https://github.com/edgeo-scada/bacnet
- **Upstream SHA:** `4a77093799a3a0e256660c6d66f12b1e918e132a`
- **Upstream commit:** *"Flatten package structure and update module path"* (2026-02-06)
- **Fork date:** 2026-05-15

From this point on, the SHA above is the base of record. Any later upstream
activity is a candidate for a future rebase or cherry-pick, not a moving target.

## Upstream activity policy

Upstream has been silent since 2026-02-06 and the author has not responded to
external issues. We will **not** track upstream on a schedule. Cherry-picks are
considered only if a meaningful upstream commit lands.

## Significant divergences

- CLI removed (`cmd/`, `examples/`); library-only.
- Dependency tree trimmed to stdlib + (optionally) `golang.org/x/sys`.
- Module path rewritten to `github.com/maestrohub-labs/bacnet-go`.
- Unit + integration tests added (see `AUDIT.md` for coverage).
- Primitives added for downstream MaestroHub use (see `CHANGELOG.md` § Added).

This file must be updated in every PR that introduces a structural change.

## Per-file copyright header policy

- Existing upstream files retain their `// Copyright 2025 Edgeo SCADA` header
  unchanged.
- Files we substantially modify get our copyright **added** as a second line;
  upstream's stays.
- New files we author carry only the maestrohub-labs header:
  ```
  // Copyright 2026 maestrohub-labs
  // Licensed under the Apache License, Version 2.0 (the "License");
  // you may not use this file except in compliance with the License.
  // You may obtain a copy of the License at
  //
  //     http://www.apache.org/licenses/LICENSE-2.0
  ```

## Maintainer / governance

- Sole admin: **@shamaan0086**. Branch protection on `main` may be added later;
  currently the repository operates on a single-admin trust model. This is an
  accepted risk for v0.1.0 and is documented here so that it is not silently
  hidden.
- License compliance (Apache-2.0 § 4) is satisfied by:
  - `LICENSE` (upstream, unchanged)
  - `NOTICE` (this fork's attribution)
  - per-file copyright headers (policy above)
  - this `FORK.md` summary of significant changes
