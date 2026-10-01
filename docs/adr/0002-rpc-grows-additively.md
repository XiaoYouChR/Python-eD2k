# ADR-0002: The RPC grows additively

## Status

Accepted

## Decision

New commands, settings fields, and snapshot fields are additions to RPC
version 1. `RPC_VERSION` changes only for a change an older caller cannot
ignore. Callers that need an addition check the `goed2kd --version` minimum
before starting the Sidecar.

## Context

Released Ghost Downloader builds install the latest `goed2kd` release. A version
bump would break every older build that installs or updates the Sidecar.

## Consequences

- Older callers keep working against a newer Sidecar: unknown request fields
  default to zero values, and unknown snapshot fields are ignored.
- A newer caller against an older Sidecar is the caller's problem to detect,
  by version, not by waiting for an unknown-method error.
