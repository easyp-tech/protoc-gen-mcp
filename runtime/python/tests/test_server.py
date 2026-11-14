import unittest

import mcp.server.lowlevel

import protoc_gen_mcp_runtime as runtime


class ConfiguredServerTest(unittest.TestCase):
    def _capabilities(self, server):
        options = server.create_initialization_options()
        return options.capabilities.model_dump(by_alias=True, exclude_none=True, mode="json")

    def test_apps_adds_the_extension(self):
        server = runtime.new_configured_server("demo", "v1.2.3", apps=True)
        self.assertEqual(server.name, "demo")
        self.assertEqual(server.version, "v1.2.3")
        self.assertEqual(
            self._capabilities(server)["extensions"],
            {runtime.APP_EXTENSION_ID: {"mimeTypes": [runtime.APP_HTML_MIME_TYPE]}},
        )

    def test_without_apps_capabilities_stay_sdk_defaults(self):
        server = runtime.new_configured_server("demo", "", apps=False)
        self.assertEqual(server.version, "v0.0.1")
        self.assertNotIn("extensions", self._capabilities(server))
        self.assertEqual(
            self._capabilities(server),
            mcp.server.lowlevel.Server("plain").create_initialization_options().capabilities.model_dump(
                by_alias=True, exclude_none=True, mode="json"
            ),
        )

    def test_name_is_required(self):
        with self.assertRaisesRegex(ValueError, "needs a name"):
            runtime.new_configured_server("", "v1", apps=False)


if __name__ == "__main__":
    unittest.main()
