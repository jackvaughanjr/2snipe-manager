# docs/

Reference documentation for the `snipemgr` manager and the `*2snipe` integration family.

| File | Audience | Purpose |
|------|----------|---------|
| [architecture.md](architecture.md) | snipemgr contributors | Component design, data types, wizard flow, dependency rationale |
| [gcp-infra.md](gcp-infra.md) | snipemgr contributors | GCP setup, IAM requirements, API references, cost estimate |
| [features-backlog.md](features-backlog.md) | snipemgr contributors | Post-core enhancement ideas, tiered by value and complexity |
| [order-of-operations.md](order-of-operations.md) | snipemgr contributors | Historical build plan, phase gotchas, and verification logs |
| [INTEGRATION_CONTRACT.md](INTEGRATION_CONTRACT.md) | both | Stable contract between `snipemgr` and any compliant `*2snipe` integration |
| [manifest-spec.md](manifest-spec.md) | both | Full spec for the `2snipe.json` manifest that makes an integration discoverable |
| [release.md](release.md) | integration authors | Versioning convention, release workflow template, badge row, and README patterns |
| [scaffolding.md](scaffolding.md) | integration authors | File structure, `go.mod`, `cmd/`, and syncer templates for new integrations |
| [source-files.md](source-files.md) | integration authors | Verbatim `snipeit` and `slack` client source to copy into new integrations |
| [snipeit-api.md](snipeit-api.md) | integration authors | Snipe-IT API reference: envelope behavior, checkout/checkin, sync flow, gotchas |

---

**Working on `snipemgr` itself?** Start with `CONTEXT.md` in the repo root, then `architecture.md`.

**Building a new `*2snipe` integration?** Start with `scaffolding.md`, then `source-files.md`, `snipeit-api.md`, `release.md`, and `manifest-spec.md`.
