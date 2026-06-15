# Python-eD2k

Python-eD2k is a typed asyncio client for
[monkeyWie/goed2k](https://github.com/monkeyWie/goed2k). It runs goed2k in one
small Go sidecar and communicates through stdio NDJSON.

The sidecar approach keeps Go's runtime, goroutines, networking, and durable
protocol state inside Go. Python owns process lifecycle and product
integration. No C ABI, ctypes, cffi, PySide6, or transport abstraction is
required.

## Responsibilities

- `Client` owns one sidecar process, request matching, and latest-value snapshot
  delivery on the caller's running asyncio loop.
- `goed2kd` owns one `goed2k.Client`, executes commands serially, and persists
  protocol state below `dataDir`.
- The caller owns retry policy, product tasks, UI state, timeouts, and the
  sidecar executable path.

## Build and test

```powershell
go build -o goed2kd.exe ./cmd/goed2kd
python -m unittest tests.test_client -v
```

On Linux or macOS, build the executable as `goed2kd`.

## Run the example

```powershell
python main.py
```

The example builds a missing sidecar, uses hard-coded Server and Kad bootstrap
sources, and downloads the example Windows ISO into `downloads/`. Protocol
state is stored in `data/`, so rerunning the script resumes the same transfer.
Press `Ctrl+C` to save state and stop.

## Use

```python
import asyncio
from pathlib import Path

from python_ed2k import Client, Settings


async def main() -> None:
    client = Client(Path("goed2kd.exe"), Path("ed2k-state"))
    await client.start(Settings(enableDht=False, enableUpnp=False))

    transfer = await client.addLink(
        "ed2k://|file|example.bin|2048|31D6CFE0D16AE931B73C59D7E0C089C0|/",
        Path("downloads"),
    )
    print(transfer)

    async for snapshot in client.snapshots():
        print(snapshot)
        break

    async with asyncio.timeout(5):
        await client.close()


asyncio.run(main())
```

`snapshots()` yields the latest full state, not an event history. Slow consumers
may skip older snapshots. `close()` has no internal timeout; use
`asyncio.timeout()` when the caller needs one, then call `terminate()` if
required.
