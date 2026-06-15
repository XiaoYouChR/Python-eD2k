# Domain Context

## Client

The Python entry point used by one application event loop. It owns one sidecar
process and hides process creation, NDJSON requests, response matching, and
snapshot delivery.

The Client does not own product tasks, retry policy, UI state, or the eD2k
protocol state. It uses the caller's running asyncio loop and never changes the
event-loop policy.

## Sidecar

The `goed2kd` process. It owns one `goed2k.Client` and all eD2k protocol
activity for the application. Commands execute serially in stdin order. Status
updates come from `goed2k.SubscribeStatus`; the Sidecar adds no polling timer.

The Sidecar does not know the caller's task model or user interface.

## Settings

The small, supported subset of engine startup settings. The caller chooses
bootstrap sources and ports. The Sidecar decides how connections are
maintained.

## Snapshot

The current observable eD2k state. Snapshots are facts, not a reliable history
of changes. Slow consumers may skip older snapshots.

## Transfer

One eD2k download identified by its file hash. Product metadata such as
category, queue position, and UI text does not belong to a Transfer.

## Durable State

Protocol state owned and persisted by `goed2k`. The caller chooses its parent
directory but does not read, write, or interpret its contents.
