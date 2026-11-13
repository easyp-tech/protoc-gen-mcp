package internal.examplemcp.java;

import com.google.protobuf.ByteString;
import io.modelcontextprotocol.json.McpJsonDefaults;
import io.modelcontextprotocol.server.transport.StdioServerTransportProvider;
import io.modelcontextprotocol.spec.McpSchema;
import internal.testproto.prompts.v1.PromptsMcp;
import internal.testproto.resources.v1.Resources;
import internal.testproto.resources.v1.ResourcesMcp;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Map;

/** Java counterpart of examplemcp.NewResourcesServer for the stdio resources contract. */
final class ResourcesJavaServer {
  private ResourcesJavaServer() {
  }

  static void run() {
    StdioServerTransportProvider transportProvider =
        new StdioServerTransportProvider(McpJsonDefaults.getMapper());
    ResourcesMcp.registerResourcesResources(transportProvider, new ResourcesHandler(), null);
    PromptsMcp.registerPromptsPrompts(transportProvider, new PromptsHandler(), null);
  }

  private static String readRepoFile(String relativePath) throws Exception {
    return Files.readString(Path.of(relativePath), StandardCharsets.UTF_8);
  }

  private static final class ResourcesHandler implements ResourcesMcp.ResourcesResourceHandler {
    @Override
    public Resources.ServerStatus readServerStatus() {
      return Resources.ServerStatus.newBuilder().setHealthy(true).setUptimeSeconds(123).setVersion("v0.0.1").build();
    }

    @Override
    public List<McpSchema.Resource> listUserProfiles() {
      return List.of(McpSchema.Resource.builder()
          .name("user_profile")
          .uri("users://ada/profile")
          .description("User profile information")
          .build());
    }

    @Override
    public Resources.UserProfile readUserProfile(String userId) {
      return Resources.UserProfile.newBuilder()
          .setUserId(userId)
          .setDisplayName("Ada Lovelace")
          .setEmail(userId + "@example.com")
          .setActive(true)
          .build();
    }

    @Override
    public List<McpSchema.Resource> listDocuments() {
      return List.of();
    }

    @Override
    public Resources.Document readDocument(String projectId, String documentId) {
      return Resources.Document.newBuilder()
          .setTitle("doc " + documentId + " in " + projectId)
          .setBody("body")
          .setAuthor("Ada")
          .build();
    }

    @Override
    public Resources.SkillDocument readSkillDocument() throws Exception {
      return Resources.SkillDocument.newBuilder()
          .setMarkdown(readRepoFile("internal/examplemcp/testdata/SKILL.md"))
          .build();
    }

    @Override
    public List<McpSchema.Resource> listPlainTexts() {
      return List.of(McpSchema.Resource.builder()
          .name("plain_text")
          .uri("docs://example/intro")
          .mimeType("text/plain")
          .build());
    }

    @Override
    public Resources.PlainText readPlainText(String name) {
      return Resources.PlainText.newBuilder().setBody("document: " + name).build();
    }

    @Override
    public Resources.RawBinary readRawBinary() {
      return Resources.RawBinary.newBuilder()
          .setData(ByteString.copyFrom(new byte[] {0, 1, 2, (byte) 255}))
          .build();
    }

    @Override
    public Object readAppPage() throws Exception {
      return readRepoFile("internal/testproto/resources/v1/testdata/report.html");
    }
  }

  private static List<Map<String, Object>> userMessage(String text) {
    return List.of(Map.of("role", "user", "content", Map.of("type", "text", "text", text)));
  }

  private static final class PromptsHandler implements PromptsMcp.PromptsPromptHandler {
    @Override
    public List<Map<String, Object>> codeReview(Map<String, String> arguments) {
      return userMessage("Review this " + arguments.get("language") + " code: " + arguments.get("code"));
    }

    @Override
    public List<Map<String, Object>> summarize(Map<String, String> arguments) {
      return userMessage("Summarize: " + arguments.get("content"));
    }

    @Override
    public List<Map<String, Object>> explainError(Map<String, String> arguments) {
      return userMessage("Explain: " + arguments.get("errorMessage"));
    }
  }
}
