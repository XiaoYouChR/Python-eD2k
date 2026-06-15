import asyncio
import shutil
import sys
from pathlib import Path

from python_ed2k import Client, Error, Settings, Snapshot, Transfer, TransferState


ROOT = Path(__file__).resolve().parent
SIDECAR = ROOT / ("goed2kd.exe" if sys.platform == "win32" else "goed2kd")
DATA_DIR = ROOT / "data"
DOWNLOAD_DIR = ROOT / "downloads"

SERVER_MET_SOURCE = "http://upd.emule-security.org/server.met"
NODES_DAT_SOURCE = "http://upd.emule-security.org/nodes.dat"

LINK = (
    "ed2k://|file|"
    "zh-cn_windows_11_consumer_editions_version_25h2_updated_may_2026_x64_dvd_ceef8999.iso"
    "|8700340224|2C26BA17D74D1A7439EBDED8D6D6F949|/"
)
FILE_NAME = "zh-cn_windows_11_consumer_editions_version_25h2_updated_may_2026_x64_dvd_ceef8999.iso"
FILE_HASH = "2C26BA17D74D1A7439EBDED8D6D6F949"
FILE_SIZE = 8_700_340_224


async def buildSidecar() -> None:
    if SIDECAR.exists():
        return
    print("Building goed2kd...")
    process = await asyncio.create_subprocess_exec(
        "go",
        "build",
        "-o",
        SIDECAR,
        "./cmd/goed2kd",
        cwd=ROOT,
    )
    if await process.wait() != 0:
        raise RuntimeError("failed to build goed2kd")


def transferByHash(snapshot: Snapshot) -> Transfer | None:
    return next((transfer for transfer in snapshot.transfers if transfer.hash == FILE_HASH), None)


def requireSpace(required: int) -> None:
    free = shutil.disk_usage(DOWNLOAD_DIR).free
    if free < required:
        raise RuntimeError(
            f"not enough free space: need {toSize(required)}, available {toSize(free)}"
        )


def toSize(value: int) -> str:
    size = float(value)
    for unit in ("B", "KiB", "MiB"):
        if size < 1024:
            return f"{size:.2f} {unit}"
        size /= 1024
    return f"{size:.2f} GiB"


def toProgress(transfer: Transfer) -> str:
    percent = transfer.done / transfer.size * 100 if transfer.size else 0
    return (
        f"{transfer.state:<19} {percent:6.2f}%  "
        f"{toSize(transfer.done)}/{toSize(transfer.size)}  "
        f"{toSize(transfer.downloadRate)}/s  peers={transfer.peers}"
    )


async def closeClient(client: Client) -> None:
    try:
        async with asyncio.timeout(10):
            await client.close()
    except (TimeoutError, Error, OSError) as error:
        print(f"\nGraceful close failed: {error}. Terminating goed2kd.", file=sys.stderr)
        await client.terminate()


async def download(client: Client, current: Snapshot) -> Path:
    transfer = transferByHash(current)
    target = DOWNLOAD_DIR / FILE_NAME

    if transfer is None:
        if target.exists():
            raise RuntimeError(f"{target} exists without durable transfer state")
        requireSpace(FILE_SIZE)
        transfer = await client.addLink(LINK, DOWNLOAD_DIR)
    else:
        requireSpace(max(0, transfer.size - transfer.done))
        if transfer.state is TransferState.PAUSED:
            transfer = await client.resume(transfer.hash)

    if transfer.state is TransferState.FINISHED:
        print(f"Finished: {transfer.path}")
        return transfer.path

    print(f"\r{toProgress(transfer):<100}", end="", flush=True)
    lastStatus = (
        transfer.state,
        int(transfer.done / transfer.size * 100) if transfer.size else 0,
    )
    async for snapshot in client.snapshots():
        transfer = transferByHash(snapshot)
        if transfer is None:
            continue

        percent = int(transfer.done / transfer.size * 100) if transfer.size else 0
        status = (transfer.state, percent)
        if status != lastStatus:
            print(f"\r{toProgress(transfer):<100}", end="", flush=True)
            lastStatus = status

        if transfer.state is TransferState.FINISHED:
            print(f"\nFinished: {transfer.path}")
            return transfer.path

    raise RuntimeError("goed2kd stopped before the download finished")


async def main() -> None:
    await buildSidecar()
    DATA_DIR.mkdir(exist_ok=True)
    DOWNLOAD_DIR.mkdir(exist_ok=True)

    client = Client(SIDECAR, DATA_DIR)
    try:
        current = await client.start(
            Settings(
                serverMetSource=SERVER_MET_SOURCE,
                nodesDatSource=NODES_DAT_SOURCE,
                enableDht=True,
                enableUpnp=True,
                reconnectToServer=True,
            )
        )
        await download(client, current)
    finally:
        await closeClient(client)


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\nStopped.")
    except (Error, OSError, RuntimeError) as error:
        raise SystemExit(f"\nError: {error}") from error
