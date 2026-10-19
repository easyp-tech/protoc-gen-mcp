"""Standard-library tests for the Go SDK gate, including a real compile failure."""

import importlib.util
import json
import os
from pathlib import Path
import shlex
import shutil
import subprocess
import tempfile
import unittest
from unittest import mock


SCRIPT = Path(__file__).with_name("check-go-sdk-compatibility.py")
SPEC = importlib.util.spec_from_file_location("sdk_compatibility", SCRIPT)
gate = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(gate)


class CompatibilityGateTest(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix="sdk-gate-test-")
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        (self.root / "go.mod").write_text("module compatibility.test\n\ngo 1.24\n")
        (self.root / "go.sum").write_text("")
        self.fixture = self.root / "internal/testproto/demo/v1/demo.mcp.go"
        self.fixture.parent.mkdir(parents=True)
        self.fixture.write_text("package demo\nvar Priority = 0.8\n")
        self.golden = self.root / "testdata/golden/demo.mcp.go.golden"
        self.golden.parent.mkdir(parents=True)
        self.header = "// source: internal/testproto/demo/v1/demo.proto\n"
        self.golden.write_text(self.header + self.fixture.read_text())

    def test_repository_goldens_cover_tools_prompts_and_resources(self):
        overlay = gate.golden_overlay(SCRIPT.parent.parent)
        names = {Path(path).name for path in overlay["Replace"].values()}
        self.assertTrue({"example.mcp.go.golden", "prompts.mcp.go.golden",
                         "resources.mcp.go.golden"}.issubset(names))

    def test_bad_golden_mapping_fails_closed(self):
        for header in ("", "// source: ../../outside.proto\n",
                       "// source: internal/testproto/missing.proto\n"):
            with self.subTest(header=header):
                self.golden.write_text(header + self.fixture.read_text())
                with self.assertRaises(ValueError):
                    gate.golden_overlay(self.root)
        self.golden.unlink()
        with self.assertRaisesRegex(ValueError, "no Go goldens"):
            gate.golden_overlay(self.root)

    def test_duplicate_golden_fails_closed(self):
        self.golden.with_name("duplicate.mcp.go.golden").write_text(self.golden.read_text())
        with self.assertRaisesRegex(ValueError, "duplicate golden"):
            gate.golden_overlay(self.root)

    def test_lanes_isolate_dependencies_and_propagate_overlay(self):
        for sdk in ("pinned", "latest"):
            with self.subTest(sdk=sdk):
                calls = []

                def run(root, env, *args, capture=False):
                    flags = shlex.split(env["GOFLAGS"])
                    modfile = Path(next(flag.split("=", 1)[1] for flag in flags
                                        if flag.startswith("-modfile=")))
                    self.assertNotEqual(modfile.parent, root)
                    self.assertTrue(modfile.is_file())
                    self.assertEqual(env["GOWORK"], "off")
                    if args[:2] == ("go", "get"):
                        modfile.write_text("// updated only in the temporary module\n")
                    calls.append((args, flags, modfile))
                    return "v1.8.0" if capture else None

                with mock.patch.object(gate, "run", side_effect=run):
                    gate.check(self.root, sdk)
                gets = [args for args, _, _ in calls if args[:2] == ("go", "get")]
                self.assertEqual(gets, [("go", "get", gate.SDK + "@latest")] if sdk == "latest" else [])
                test_calls = [(args, flags) for args, flags, _ in calls if args[:2] == ("go", "test")]
                self.assertEqual(len(test_calls), 4)
                self.assertFalse(any(flag.startswith("-overlay=") for flag in test_calls[0][1]))
                for _, flags in test_calls[2:]:
                    self.assertTrue(any(flag.startswith("-overlay=") for flag in flags))
                    self.assertIn("-mod=readonly", flags)
                self.assertIn(gate.SMOKE_TESTS, test_calls[-1][0])
                self.assertTrue(all(not modfile.exists() for _, _, modfile in calls))
                self.assertEqual((self.root / "go.mod").read_text(), "module compatibility.test\n\ngo 1.24\n")
                self.assertEqual((self.root / "go.sum").read_text(), "")

    def test_latest_download_failure_is_not_silently_skipped(self):
        with mock.patch.object(gate, "run", side_effect=("v1.8.0", subprocess.CalledProcessError(1, "go get"))) as run:
            with self.assertRaises(subprocess.CalledProcessError):
                gate.check(self.root, "latest")
            self.assertEqual(run.call_count, 2)

    @unittest.skipUnless(shutil.which("go"), "Go toolchain is required for the compile regression")
    def test_overlay_compiles_actual_golden_bytes(self):
        # The fixture remains valid. Only the golden has a type error. This
        # catches accidentally testing fixtures or comparing strings instead
        # of compiling the golden source itself.
        self.golden.write_text(self.header + "package demo\nvar Priority = missingSDKField\n")
        overlay = self.root / "overlay.json"
        overlay.write_text(json.dumps(gate.golden_overlay(self.root)))
        env = dict(os.environ, GOWORK="off", GOFLAGS="-mod=readonly")
        command = ["go", "test", "./internal/testproto/..."]
        fixture = subprocess.run(command, cwd=self.root, env=env, capture_output=True, text=True)
        self.assertEqual(fixture.returncode, 0, fixture.stdout + fixture.stderr)
        env["GOFLAGS"] = shlex.join(["-mod=readonly", f"-overlay={overlay}"])
        golden = subprocess.run(command, cwd=self.root, env=env, capture_output=True, text=True)
        self.assertNotEqual(golden.returncode, 0)
        self.assertIn("missingSDKField", golden.stdout + golden.stderr)


if __name__ == "__main__":
    unittest.main()
