import assert from "node:assert/strict";
import { test } from "node:test";

import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { McpError } from "@modelcontextprotocol/sdk/types.js";

import {
  GENCODE_VERSION,
  addTools,
  checkGencodeVersion,
  compileUriTemplate,
  matchUriTemplate,
  resolveToolName,
} from "../dist/index.js";

function template(uriTemplate, ...names) {
  return { pattern: compileUriTemplate(uriTemplate), names };
}

test("URI templates match and percent-decode like the Go runtime", () => {
  const entry = template("users://{user_id}/profile", "user_id");
  assert.deepEqual(matchUriTemplate(entry, "users://ada%20l/profile"), { user_id: "ada l" });
  assert.deepEqual(matchUriTemplate(entry, "users://a+b/profile"), { user_id: "a+b" });
  assert.equal(matchUriTemplate(entry, "users://ada/settings"), undefined);
  assert.equal(matchUriTemplate(entry, "users://a/b/profile"), undefined);
  assert.throws(() => matchUriTemplate(entry, "users://bad%zz/profile"), McpError);

  const dotted = template("docs://v1.0/{name}", "name");
  assert.equal(matchUriTemplate(dotted, "docs://v1x0/a"), undefined);
});

test("tool names join namespace and method without dots", () => {
  assert.equal(resolveToolName("raw.pb", "Create", "ignored"), "raw_pb_Create");
  assert.equal(resolveToolName(undefined, "Create", "example.default"), "example_default_Create");
  assert.equal(resolveToolName("", "Create", "example"), "Create");
});

test("tool names are unique per server across generated files", () => {
  const tool = { name: "demo_echo", icons: [] };
  const server = new Server({ name: "runtime-test", version: "0.0.0" }, { capabilities: { tools: {} } });
  addTools(server, [tool]);
  assert.throws(() => addTools(server, [tool]), /duplicate tool registration: demo_echo/);
  const other = new Server({ name: "other", version: "0.0.0" }, { capabilities: { tools: {} } });
  addTools(other, [tool]);
});

test("generated code with another runtime API version fails to load", () => {
  checkGencodeVersion(GENCODE_VERSION);
  assert.throws(() => checkGencodeVersion(GENCODE_VERSION + 1), /regenerate the code/);
});
