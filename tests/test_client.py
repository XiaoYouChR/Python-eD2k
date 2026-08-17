import asyncio
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from python_ed2k import Client, Error, ErrorCode, Settings


ROOT = Path(__file__).parents[1]
SIDECAR = ROOT / ("goed2kd.exe" if sys.platform == "win32" else "goed2kd")


class ClientTest(unittest.IsolatedAsyncioTestCase):
    @classmethod
    def setUpClass(cls) -> None:
        subprocess.run(
            ["go", "build", "-o", SIDECAR, "./cmd/goed2kd"],
            cwd=ROOT,
            check=True,
        )

    async def test_client_starts_with_an_empty_snapshot_and_closes(self) -> None:
        with tempfile.TemporaryDirectory() as dataDir:
            client = Client(SIDECAR, Path(dataDir))

            started = await client.start(Settings(enableDht=False, enableUpnp=False))
            self.addAsyncCleanup(client.terminate)
            current = await client.snapshot()

            self.assertEqual(started.transfers, ())
            self.assertEqual(current, started)

            await asyncio.wait_for(client.close(), timeout=5)

    async def test_client_manages_a_transfer_lifecycle(self) -> None:
        link = "ed2k://|file|example.bin|2048|31D6CFE0D16AE931B73C59D7E0C089C0|/"
        with tempfile.TemporaryDirectory() as dataDir, tempfile.TemporaryDirectory() as outputDir:
            client = Client(SIDECAR, Path(dataDir))
            await client.start(Settings(enableDht=False, enableUpnp=False))
            self.addAsyncCleanup(client.terminate)

            added = await client.addLink(link, Path(outputDir))
            paused = await client.pause(added.hash)
            resumed = await client.resume(added.hash)
            await client.remove(added.hash)

            self.assertEqual(added.name, "example.bin")
            self.assertEqual(added.size, 2048)
            self.assertEqual(added.activePeers, 0)
            self.assertEqual(added.peers, 0)
            self.assertEqual(paused.state, "PAUSED")
            self.assertEqual(resumed.state, "DOWNLOADING")
            self.assertEqual((await client.snapshot()).transfers, ())

            await asyncio.wait_for(client.close(), timeout=5)

    async def test_remove_reports_transfer_not_found(self) -> None:
        with tempfile.TemporaryDirectory() as dataDir:
            client = Client(SIDECAR, Path(dataDir))
            await client.start(Settings(enableDht=False, enableUpnp=False))
            self.addAsyncCleanup(client.terminate)

            with self.assertRaises(Error) as raised:
                await client.remove("31D6CFE0D16AE931B73C59D7E0C089C0")

            self.assertEqual(raised.exception.code, ErrorCode.TRANSFER_NOT_FOUND)

            await asyncio.wait_for(client.close(), timeout=5)

    async def test_add_link_rejects_an_output_path_conflict(self) -> None:
        first = "ed2k://|file|same.bin|2048|31D6CFE0D16AE931B73C59D7E0C089C0|/"
        second = "ed2k://|file|same.bin|2048|31D6CFE0D14CE931B73C59D7E0C04BC0|/"
        with tempfile.TemporaryDirectory() as dataDir, tempfile.TemporaryDirectory() as outputDir:
            client = Client(SIDECAR, Path(dataDir))
            await client.start(Settings(enableDht=False, enableUpnp=False))
            self.addAsyncCleanup(client.terminate)
            await client.addLink(first, Path(outputDir))

            with self.assertRaises(Error) as raised:
                await client.addLink(second, Path(outputDir))

            self.assertEqual(raised.exception.code, ErrorCode.OUTPUT_EXISTS)

            await asyncio.wait_for(client.close(), timeout=5)

    async def test_add_link_rejects_an_existing_output_file(self) -> None:
        link = "ed2k://|file|existing.bin|2048|31D6CFE0D16AE931B73C59D7E0C089C0|/"
        with tempfile.TemporaryDirectory() as dataDir, tempfile.TemporaryDirectory() as outputDir:
            Path(outputDir, "existing.bin").write_bytes(b"unrelated")
            client = Client(SIDECAR, Path(dataDir))
            await client.start(Settings(enableDht=False, enableUpnp=False))
            self.addAsyncCleanup(client.terminate)

            with self.assertRaises(Error) as raised:
                await client.addLink(link, Path(outputDir))

            self.assertEqual(raised.exception.code, ErrorCode.OUTPUT_EXISTS)

            await asyncio.wait_for(client.close(), timeout=5)

    async def test_snapshots_yields_the_latest_state(self) -> None:
        link = "ed2k://|file|event.bin|4096|31D6CFE0D14CE931B73C59D7E0C04BC0|/"
        with tempfile.TemporaryDirectory() as dataDir, tempfile.TemporaryDirectory() as outputDir:
            client = Client(SIDECAR, Path(dataDir))
            await client.start(Settings(enableDht=False, enableUpnp=False))
            self.addAsyncCleanup(client.terminate)
            snapshots = client.snapshots()

            initial = await asyncio.wait_for(anext(snapshots), timeout=2)
            added = await client.addLink(link, Path(outputDir))
            changed = await asyncio.wait_for(anext(snapshots), timeout=2)

            self.assertEqual(initial.transfers, ())
            self.assertEqual(changed.transfers, (added,))

            await snapshots.aclose()
            await asyncio.wait_for(client.close(), timeout=5)

    async def test_client_can_restart_on_the_same_event_loop(self) -> None:
        with tempfile.TemporaryDirectory() as dataDir:
            client = Client(SIDECAR, Path(dataDir))
            self.addAsyncCleanup(client.terminate)

            await client.start(Settings(enableDht=False, enableUpnp=False))
            await asyncio.wait_for(client.close(), timeout=5)
            restarted = await client.start(Settings(enableDht=False, enableUpnp=False))

            self.assertEqual(restarted.transfers, ())

            await asyncio.wait_for(client.close(), timeout=5)

    async def test_bootstrap_source_preserves_an_http_url(self) -> None:
        source = "http://127.0.0.1:1/server.met"
        with tempfile.TemporaryDirectory() as dataDir:
            client = Client(SIDECAR, Path(dataDir))
            self.addAsyncCleanup(client.terminate)

            with self.assertRaises(Error) as raised:
                await client.start(
                    Settings(
                        serverMetSource=source,
                        enableDht=False,
                        enableUpnp=False,
                    )
                )

            self.assertIn(source, str(raised.exception))

    async def test_dht_bootstrap_source_preserves_an_http_url(self) -> None:
        source = "http://127.0.0.1:1/nodes.dat"
        with tempfile.TemporaryDirectory() as dataDir:
            client = Client(SIDECAR, Path(dataDir))
            self.addAsyncCleanup(client.terminate)

            with self.assertRaises(Error) as raised:
                await client.start(
                    Settings(
                        nodesDatSource=source,
                        enableDht=False,
                        enableUpnp=False,
                    )
                )

            self.assertIn(source, str(raised.exception))
