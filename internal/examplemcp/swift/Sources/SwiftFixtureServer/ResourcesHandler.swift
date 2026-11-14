import Foundation
import MCP
import ProtocGenMCPRuntime

/// Swift counterpart of examplemcp.NewResourcesServer for the stdio resources contract.
struct ResourcesHandler: ResourcesResourceHandler {
    func readServerStatus() async throws -> Internal_Testproto_Resources_V1_ServerStatus {
        var status = Internal_Testproto_Resources_V1_ServerStatus()
        status.healthy = true
        status.uptimeSeconds = 123
        status.version = "v0.0.1"
        return status
    }

    func listUserProfiles() async throws -> [Resource] {
        [Resource(name: "user_profile", uri: "users://ada/profile", description: "User profile information")]
    }

    func readUserProfile(userID: String) async throws -> Internal_Testproto_Resources_V1_UserProfile {
        var profile = Internal_Testproto_Resources_V1_UserProfile()
        profile.userID = userID
        profile.displayName = "Ada Lovelace"
        profile.email = "\(userID)@example.com"
        profile.active = true
        return profile
    }

    func listDocuments() async throws -> [Resource] {
        []
    }

    func readDocument(projectID: String, documentID: String) async throws -> Internal_Testproto_Resources_V1_Document {
        var document = Internal_Testproto_Resources_V1_Document()
        document.title = "doc \(documentID) in \(projectID)"
        document.body = "body"
        document.author = "Ada"
        return document
    }

    func readSkillDocument() async throws -> Internal_Testproto_Resources_V1_SkillDocument {
        var skill = Internal_Testproto_Resources_V1_SkillDocument()
        skill.markdown = try String(contentsOfFile: "internal/examplemcp/testdata/SKILL.md", encoding: .utf8)
        return skill
    }

    func listPlainTexts() async throws -> [Resource] {
        [Resource(name: "plain_text", uri: "docs://example/intro", mimeType: "text/plain")]
    }

    func readPlainText(name: String) async throws -> Internal_Testproto_Resources_V1_PlainText {
        var text = Internal_Testproto_Resources_V1_PlainText()
        text.body = "document: \(name)"
        return text
    }

    func readRawBinary() async throws -> Internal_Testproto_Resources_V1_RawBinary {
        var binary = Internal_Testproto_Resources_V1_RawBinary()
        binary.data = Data([0, 1, 2, 255])
        return binary
    }

    func readAppPage() async throws -> RawResourceBody {
        .text(try String(contentsOfFile: "internal/testproto/resources/v1/testdata/report.html", encoding: .utf8))
    }
}

struct PromptsHandler: PromptsPromptHandler {
    private func user(_ text: String) -> GetPrompt.Result {
        GetPrompt.Result(description: nil, messages: [Prompt.Message(role: .user, content: .text(text: text))])
    }

    func codeReview(_ arguments: [String: String]) async throws -> GetPrompt.Result {
        user("Review this \(arguments["language"] ?? "") code: \(arguments["code"] ?? "")")
    }

    func summarize(_ arguments: [String: String]) async throws -> GetPrompt.Result {
        user("Summarize: \(arguments["content"] ?? "")")
    }

    func explainError(_ arguments: [String: String]) async throws -> GetPrompt.Result {
        user("Explain: \(arguments["errorMessage"] ?? "")")
    }
}
