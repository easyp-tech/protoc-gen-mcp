#!/usr/bin/env python3
"""Compile checked-in Go fixtures/goldens against the pinned or latest SDK.

Only the Python standard library and Go are required. An isolated -modfile and
Go source overlay leave the checkout (including go.mod/go.sum) untouched. The
overlay compiles the actual .golden bytes in place of their fixture sidecars;
it is also inherited by the stdio test's child `go run` process.
"""

import argparse
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import subprocess
import tempfile


SDK = "github.com/modelcontextprotocol/go-sdk"
SMOKE_TESTS = "^(" + "|".join((
    "TestServerOverStdio",
    "TestResourcesPromptsRoundTrip",
    "TestGeneratedAppUIFromProto",
    "TestAppUIUsesProtobufResource",
    "TestPromptsGetMissingRequiredArg",
    "TestGeneratedMCPServerAndOAuthHTTPFactory",
    "TestGeneratedAnnotationsSDKCompatibility",
)) + ")$"


def golden_overlay(root):
    """Map every checked-in Go golden to its real, existing generated fixture."""
    root = root.resolve()
    replacements = {}
    for golden in sorted((root / "testdata/golden").glob("*.mcp.go.golden")):
        source = re.search(r"^// source: (.+\.proto)$", golden.read_text(), re.M)
        if source is None:
            raise ValueError(f"missing protobuf source header: {golden}")
        fixture = (root / source.group(1)).with_suffix(".mcp.go").resolve()
        if not fixture.is_relative_to(root / "internal/testproto"):
            raise ValueError(f"golden is not a test fixture: {golden}")
        if not fixture.is_file():
            raise ValueError(f"missing generated fixture for {golden}: {fixture}")
        if str(fixture) in replacements:
            raise ValueError(f"duplicate golden for {fixture}")
        replacements[str(fixture)] = str(golden)
    if not replacements:
        raise ValueError("no Go goldens found; refusing an empty compatibility gate")
    return {"Replace": replacements}


def run(root, env, *args, capture=False):
    print("+ " + shlex.join(args), flush=True)
    result = subprocess.run(args, cwd=root, env=env, check=True,
                            text=True, stdout=subprocess.PIPE if capture else None)
    return result.stdout.strip() if capture else None


def check(root, sdk):
    root = root.resolve()
    overlay = golden_overlay(root)
    with tempfile.TemporaryDirectory(prefix="protoc-gen-mcp-sdk-") as temporary:
        temporary = Path(temporary)
        modfile = temporary / "go.mod"
        shutil.copyfile(root / "go.mod", modfile)
        shutil.copyfile(root / "go.sum", temporary / "go.sum")
        overlay_file = temporary / "overlay.json"
        overlay_file.write_text(json.dumps(overlay))

        # Do not inherit an enclosing workspace, vendor mode, or caller's source
        # overlay: this gate must test the selected dependency and golden bytes.
        env = dict(os.environ, GOWORK="off", GOFLAGS=shlex.join([f"-modfile={modfile}"]))
        pinned = run(root, env, "go", "list", "-m", "-f={{.Version}}", SDK, capture=True)
        if sdk == "latest":
            # @latest resolves the newest published stable release on each run.
            # Do not guess a future version or silently fall back on fetch errors.
            run(root, env, "go", "get", SDK + "@latest")
        resolved = run(root, env, "go", "list", "-m", "-f={{.Version}}", SDK, capture=True)
        print(f"SDK lane={sdk}: resolved={resolved}, repository pin={pinned}", flush=True)
        if sdk == "latest" and resolved == pinned:
            print("The repository pin is already the latest published SDK release.", flush=True)

        flags = [f"-modfile={modfile}", "-mod=readonly"]
        env["GOFLAGS"] = shlex.join(flags)
        # Compile all checked-in generated Go packages, including the OAuth
        # server/HTTP factory fixture that does not have a separate golden.
        run(root, env, "go", "test", "-count=1", "-timeout=90s", "./internal/testproto/...")
        run(root, env, "go", "test", "-v", "-count=1", "-timeout=90s", "./mcpruntime")

        env["GOFLAGS"] = shlex.join(flags + [f"-overlay={overlay_file}"])
        for fixture, golden in overlay["Replace"].items():
            print(f"Compile golden: {Path(golden).relative_to(root)} -> "
                  f"{Path(fixture).relative_to(root)}", flush=True)
        run(root, env, "go", "test", "-count=1", "-timeout=90s", "./internal/testproto/...")
        # Real SDK sessions exercise tools, annotations, prompts, resources,
        # embedded Apps assets, and both generated server/HTTP factory paths.
        run(root, env, "go", "test", "-v", "-count=1", "-timeout=90s",
            "./internal/examplemcp", "-run", SMOKE_TESTS)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--sdk", choices=("pinned", "latest"), default="pinned")
    arguments = parser.parse_args()
    check(Path(__file__).resolve().parent.parent, arguments.sdk)
