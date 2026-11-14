package tech.easyp.mcp.runtime.oauth;

import java.math.BigDecimal;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * A strict JSON reader for MCP request bodies and JWKS documents: exactly one
 * value, no trailing data, and optionally no repeated "name" key in an object,
 * which a lenient parser would silently collapse.
 */
public final class Json {
  private final String text;
  private final boolean rejectDuplicateName;
  private int index;

  private Json(String text, boolean rejectDuplicateName) {
    this.text = text;
    this.rejectDuplicateName = rejectDuplicateName;
  }

  /** Parses text into Map, List, String, BigDecimal, Boolean or null. */
  public static Object parse(String text, boolean rejectDuplicateName) {
    Json json = new Json(text, rejectDuplicateName);
    json.skipWhitespace();
    Object value = json.value();
    json.skipWhitespace();
    if (json.index != text.length()) {
      throw new IllegalArgumentException("unexpected data after the JSON value");
    }
    return value;
  }

  private Object value() {
    if (index >= text.length()) {
      throw new IllegalArgumentException("unexpected end of JSON");
    }
    char c = text.charAt(index);
    switch (c) {
      case '{':
        return object();
      case '[':
        return array();
      case '"':
        return string();
      case 't':
        return literal("true", Boolean.TRUE);
      case 'f':
        return literal("false", Boolean.FALSE);
      case 'n':
        return literal("null", null);
      default:
        return number();
    }
  }

  private Map<String, Object> object() {
    index++;
    Map<String, Object> result = new LinkedHashMap<>();
    skipWhitespace();
    if (peek() == '}') {
      index++;
      return result;
    }
    while (true) {
      skipWhitespace();
      if (peek() != '"') {
        throw new IllegalArgumentException("expected an object key");
      }
      String key = string();
      if (result.containsKey(key) && rejectDuplicateName && key.equals("name")) {
        throw new IllegalArgumentException("ambiguous duplicate tool name");
      }
      skipWhitespace();
      expect(':');
      skipWhitespace();
      result.put(key, value());
      skipWhitespace();
      char c = next();
      if (c == '}') {
        return result;
      }
      if (c != ',') {
        throw new IllegalArgumentException("expected , or } in object");
      }
    }
  }

  private List<Object> array() {
    index++;
    List<Object> result = new ArrayList<>();
    skipWhitespace();
    if (peek() == ']') {
      index++;
      return result;
    }
    while (true) {
      skipWhitespace();
      result.add(value());
      skipWhitespace();
      char c = next();
      if (c == ']') {
        return result;
      }
      if (c != ',') {
        throw new IllegalArgumentException("expected , or ] in array");
      }
    }
  }

  private String string() {
    expect('"');
    StringBuilder out = new StringBuilder();
    while (true) {
      char c = next();
      if (c == '"') {
        return out.toString();
      }
      if (c < 0x20) {
        throw new IllegalArgumentException("control character in string");
      }
      if (c != '\\') {
        out.append(c);
        continue;
      }
      char escape = next();
      switch (escape) {
        case '"', '\\', '/' -> out.append(escape);
        case 'b' -> out.append('\b');
        case 'f' -> out.append('\f');
        case 'n' -> out.append('\n');
        case 'r' -> out.append('\r');
        case 't' -> out.append('\t');
        case 'u' -> {
          if (index + 4 > text.length()) {
            throw new IllegalArgumentException("truncated unicode escape");
          }
          out.append((char) Integer.parseInt(text.substring(index, index + 4), 16));
          index += 4;
        }
        default -> throw new IllegalArgumentException("invalid escape");
      }
    }
  }

  private Object literal(String word, Object value) {
    if (!text.startsWith(word, index)) {
      throw new IllegalArgumentException("invalid literal");
    }
    index += word.length();
    return value;
  }

  private BigDecimal number() {
    int start = index;
    while (index < text.length() && "+-0123456789.eE".indexOf(text.charAt(index)) >= 0) {
      index++;
    }
    if (start == index) {
      throw new IllegalArgumentException("unexpected character in JSON");
    }
    return new BigDecimal(text.substring(start, index));
  }

  private void skipWhitespace() {
    while (index < text.length() && " \t\r\n".indexOf(text.charAt(index)) >= 0) {
      index++;
    }
  }

  private char peek() {
    if (index >= text.length()) {
      throw new IllegalArgumentException("unexpected end of JSON");
    }
    return text.charAt(index);
  }

  private char next() {
    char c = peek();
    index++;
    return c;
  }

  private void expect(char c) {
    if (next() != c) {
      throw new IllegalArgumentException("expected " + c);
    }
  }

  /** Writes a JSON string literal. */
  public static String quote(String value) {
    StringBuilder out = new StringBuilder("\"");
    for (char c : value.toCharArray()) {
      switch (c) {
        case '"' -> out.append("\\\"");
        case '\\' -> out.append("\\\\");
        case '\n' -> out.append("\\n");
        case '\r' -> out.append("\\r");
        case '\t' -> out.append("\\t");
        default -> {
          if (c < 0x20) {
            out.append(String.format("\\u%04x", (int) c));
          } else {
            out.append(c);
          }
        }
      }
    }
    return out.append('"').toString();
  }
}
