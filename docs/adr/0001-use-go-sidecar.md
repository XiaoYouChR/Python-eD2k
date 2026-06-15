# ADR-0001: Use a Go sidecar

## Status

Accepted

## Decision

Python starts one `goed2kd` process and communicates through stdio NDJSON.
`goed2kd` owns one `goed2k.Client` for all transfers.

The Python Client exposes typed snapshots and commands. It does not expose raw
JSON, Go objects, transport seams, or protocol persistence details.

## Consequences

- A sidecar failure does not terminate the Python application.
- The caller explicitly decides whether to restart a failed sidecar.
- IPC carries commands and snapshots, never file payloads.
- The current asyncio loop owns subprocess execution.
- The same Client instance remains bound to one event loop.
- Commands execute serially in sidecar stdin order.
- Snapshot delivery is latest-value; slow consumers may skip older snapshots.
