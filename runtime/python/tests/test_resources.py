import unittest

import mcp.shared.exceptions

import protoc_gen_mcp_runtime as runtime


def _template(uri_template, *names):
    return runtime.RegisteredResourceTemplate(
        template=None,
        pattern=runtime.compile_uri_template(uri_template),
        names=names,
        lister=None,
        reader=None,
    )


class URITemplateTest(unittest.TestCase):
    def test_matches_and_percent_decodes_like_go(self):
        entry = _template("users://{user_id}/profile", "user_id")
        self.assertEqual(runtime.match_uri_template(entry, "users://ada%20l/profile"), {"user_id": "ada l"})
        self.assertEqual(runtime.match_uri_template(entry, "users://a+b/profile"), {"user_id": "a+b"})
        self.assertIsNone(runtime.match_uri_template(entry, "users://ada/settings"))
        self.assertIsNone(runtime.match_uri_template(entry, "users://a/b/profile"))

    def test_rejects_malformed_escapes(self):
        entry = _template("docs://{name}", "name")
        for uri in ("docs://bad%zz", "docs://bad%", "docs://%ff"):
            with self.subTest(uri=uri), self.assertRaises(mcp.shared.exceptions.McpError):
                runtime.match_uri_template(entry, uri)

    def test_literal_parts_are_escaped(self):
        entry = _template("docs://v1.0/{name}", "name")
        self.assertIsNone(runtime.match_uri_template(entry, "docs://v1x0/a"))
        self.assertEqual(runtime.match_uri_template(entry, "docs://v1.0/a"), {"name": "a"})


if __name__ == "__main__":
    unittest.main()
