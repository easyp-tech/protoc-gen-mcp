package tech.easyp.mcp.runtime.oauth;

import java.util.Collection;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;
import java.util.function.Function;

/**
 * Authorization preflight ported from mcpruntime/oauth_scope.go: finds the
 * scopes every tools/call in a request body needs before the SDK dispatches
 * any of it. The SDK still parses, validates and executes accepted messages.
 */
public final class ScopeGate {
  /** The request body limit of the Go SDK, applied before reading further. */
  public static final int MAX_REQUEST_BODY_BYTES = 4 << 20;

  private ScopeGate() {
  }

  /**
   * Scopes the tools/call messages in body require, including every message
   * of a batch. Throws IllegalArgumentException for a body that is not exactly
   * one JSON value, a tools/call without object params or string name, or an
   * ambiguous duplicate "name" key.
   */
  public static Set<String> requestedToolScopes(String body, Function<String, Collection<String>> toolScopes) {
    Object payload = Json.parse(body, true);
    List<?> messages = payload instanceof List<?> batch ? batch : List.of(payload);
    Set<String> scopes = new LinkedHashSet<>();
    for (Object raw : messages) {
      if (!(raw instanceof Map<?, ?> message) || !"tools/call".equals(message.get("method")) || !message.containsKey("id")) {
        continue;
      }
      if (!(message.get("params") instanceof Map<?, ?> params)) {
        throw new IllegalArgumentException("tools/call params must be an object");
      }
      if (!(params.get("name") instanceof String name)) {
        throw new IllegalArgumentException("tools/call name must be a string");
      }
      Collection<String> required = toolScopes.apply(name);
      if (required != null) {
        scopes.addAll(required);
      }
    }
    return scopes;
  }

  /** Sorted scopes from required that granted lacks. */
  public static List<String> missing(Collection<String> required, Collection<String> granted) {
    TreeSet<String> missing = new TreeSet<>(required);
    missing.removeAll(granted);
    return List.copyOf(missing);
  }

  /**
   * Text of the in-tool guard result, or null when the call may run: a scoped
   * tool needs a token (stdio has none) holding every required scope.
   */
  public static String toolGuardError(Collection<String> required, AccessToken token) {
    if (required == null || required.isEmpty()) {
      return null;
    }
    if (token == null) {
      return "permission denied: an authenticated token is required";
    }
    for (String scope : required) {
      if (!token.scopes().contains(scope)) {
        return "permission denied: missing required scope " + Json.quote(scope);
      }
    }
    return null;
  }
}
