package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type testBearerRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (t testBearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	copy := req.Clone(req.Context())
	copy.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(copy)
}

func connectDemoClient(t *testing.T, endpoint, token string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "example-integration-test", Version: "v1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: endpoint,
		HTTPClient: &http.Client{Timeout: 5*time.Second, Transport: testBearerRoundTripper{
			token:token, base:http.DefaultTransport,
		}},
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil { t.Fatalf("connect MCP client: %v", err) }
	t.Cleanup(func(){ _ = session.Close() })
	return session
}

func TestRunnableShowcaseAllFeatures(t *testing.T) {
	ctx:=context.Background()
	server, err := newShowcaseServer(ctx)
	if err != nil {t.Fatalf("generated MCP server: %v",err)}
	handler, err := newShowcaseHTTPHandler(server, demoVerifier)
	if err != nil {t.Fatalf("generated HTTP handler: %v",err)}
	srv := httptest.NewServer(handler)
	defer srv.Close()

	metadata := httptest.NewRecorder()
	handler.ServeHTTP(metadata,httptest.NewRequest(http.MethodGet,"/.well-known/oauth-protected-resource",nil))
	if metadata.Code != http.StatusOK {t.Fatalf("OAuth metadata returned %d",metadata.Code)}
	if !bytes.Contains(metadata.Body.Bytes(),[]byte("https://auth.example.com")){
		t.Fatalf("OAuth discovery missed authorized server: %s",metadata.Body.String())
	}

	anon := httptest.NewRecorder()
	handler.ServeHTTP(anon,httptest.NewRequest(http.MethodGet,"/mcp",nil))
	if anon.Code != http.StatusUnauthorized {t.Fatalf("anonymous OAuth status %d",anon.Code)}
	if !strings.Contains(anon.Header().Get("WWW-Authenticate"),"resource_metadata="){
		t.Fatalf("missing OAuth resource metadata challenge: %s",anon.Header().Get("WWW-Authenticate"))
	}

	read := connectDemoClient(t,srv.URL+"/mcp","demo-read")
	init := read.InitializeResult()
	if init == nil || init.Capabilities == nil || init.Capabilities.Extensions["io.modelcontextprotocol/ui"] == nil {
		t.Fatalf("Apps extension not advertised: %+v",init)
	}
	tools,err := read.ListTools(ctx,nil)
	if err!=nil {t.Fatalf("list tools: %v",err)}
	if len(tools.Tools)!=2 {t.Fatalf("listed tools %d, want 2",len(tools.Tools))}
	uiFound:=false
	for _, tool := range tools.Tools {
		if tool.Name=="showcase_get_overview" {
			ui,ok:=tool.Meta["ui"].(map[string]any)
			uiFound=ok && ui["resourceUri"]=="ui://showcase/dashboard"
		}
	}
	if !uiFound {t.Fatal("protobuf tool _meta.ui missing")}
	overview,err:=read.CallTool(ctx,&mcp.CallToolParams{
		Name:"showcase_get_overview",Arguments:map[string]any{"topic":"MCP Apps"},
	})
	if err!=nil || overview.IsError {t.Fatalf("overview tool: %+v, %v",overview,err)}
	payload,_:=json.Marshal(overview.StructuredContent)
	if !bytes.Contains(payload,[]byte("MCP Apps")) {t.Fatalf("ProtoJSON result missing topic: %s",payload)}

	prompts,err:=read.ListPrompts(ctx,nil)
	if err!=nil || len(prompts.Prompts)!=1 || prompts.Prompts[0].Name!="showcase_guide" {
		t.Fatalf("prompts/list: %+v, %v",prompts,err)
	}
	guide,err:=read.GetPrompt(ctx,&mcp.GetPromptParams{
		Name:"showcase_guide",Arguments:map[string]string{"topic":"protobuf"},
	})
	if err!=nil || len(guide.Messages)!=1 {t.Fatalf("prompts/get: %+v, %v",guide,err)}

	resourceList,err:=read.ListResources(ctx,nil)
	if err!=nil {t.Fatalf("resources/list: %v",err)}
	seen:=map[string]bool{}
	for _, resource:=range resourceList.Resources {seen[resource.URI]=true}
	for _,uri:=range []string{"showcase://status","showcase://SKILL.md","ui://showcase/dashboard","showcase://attachment","showcase://notes/intro"}{
		if !seen[uri] {t.Errorf("resources/list missing %s",uri)}
	}
	templates,err:=read.ListResourceTemplates(ctx,nil)
	if err!=nil {t.Fatal(err)}
	if len(templates.ResourceTemplates)!=1 || templates.ResourceTemplates[0].URITemplate!="showcase://notes/{id}" {
		t.Fatalf("resource URI template: %+v",templates.ResourceTemplates)
	}

	type expectation struct{uri,mime,substring string}
	for _, tc := range []expectation{
		{"showcase://status","application/json",`"ready":true`},
		{"showcase://notes/intro","text/plain","plain text, not a JSON wrapper"},
		{"showcase://SKILL.md","text/markdown","# Protobuf-first MCP showcase skill"},
		{"ui://showcase/dashboard","text/html;profile=mcp-app","Protobuf-first MCP Showcase"},
	} {
		r,err:=read.ReadResource(ctx,&mcp.ReadResourceParams{URI:tc.uri})
		if err!=nil || len(r.Contents)!=1 {t.Fatalf("resources/read %s: %+v, %v",tc.uri,r,err)}
		content:=r.Contents[0]
		if content.MIMEType!=tc.mime || !strings.Contains(content.Text,tc.substring){
			t.Errorf("resource %s: MIME=%s text=%q",tc.uri,content.MIMEType,content.Text)
		}
		if tc.uri=="ui://showcase/dashboard" {
			ui,ok:=content.Meta["ui"].(map[string]any)
			if !ok {t.Fatalf("embedded HTML missing _meta.ui: %+v",content.Meta)}
			csp,ok:=ui["csp"].(map[string]any)
			if !ok || csp["connectDomains"]==nil || ui["permissions"]==nil || ui["prefersBorder"]!=true {
				t.Fatalf("embedded HTML CSP/permissions: %+v",ui)
			}
		}
	}
	bin,err:=read.ReadResource(ctx,&mcp.ReadResourceParams{URI:"showcase://attachment"})
	if err!=nil || len(bin.Contents)!=1 || !bytes.Equal(bin.Contents[0].Blob,[]byte{0,1,255,16}){
		t.Fatalf("binary MCP blob: %+v, %v",bin,err)
	}

	denied,err:=read.CallTool(ctx,&mcp.CallToolParams{
		Name:"showcase_publish_report",Arguments:map[string]any{"title":"Example","body":"private"},
	})
	if err!=nil || !denied.IsError {t.Fatalf("read-only token did not deny publish: %+v, %v",denied,err)}
	write:=connectDemoClient(t,srv.URL+"/mcp","demo-write")
	allowed,err:=write.CallTool(ctx,&mcp.CallToolParams{
		Name:"showcase_publish_report",Arguments:map[string]any{"title":"Example","body":"private"},
	})
	if err!=nil || allowed.IsError {t.Fatalf("write scope rejected: %+v, %v",allowed,err)}
	status,err:=read.ReadResource(ctx,&mcp.ReadResourceParams{URI:"showcase://status"})
	if err!=nil || !strings.Contains(status.Contents[0].Text,`"reportCount":1`){
		t.Fatalf("post-publish server state: %+v, %v",status,err)
	}
}

func TestDemoTokensAndLoopbackSafety(t *testing.T) {
	for _,tc:=range []struct{addr string;want bool}{
		{"127.0.0.1:8080",true},{"[::1]:8080",true},{"localhost:8080",true},
		{"0.0.0.0:8080",false},{":8080",false},{"192.0.2.1:8080",false},
		{"example.com:8080",false},{"bad address",false},
	}{
		if got:=demoHTTPAddressIsLoopback(tc.addr);got!=tc.want {
			t.Errorf("demoHTTPAddressIsLoopback(%q) = %t, want %t",tc.addr,got,tc.want)
		}
	}
	for _,tc:=range []struct{token string;write bool;valid bool}{
		{"demo-read",false,true},{"demo-write",true,true},{"wrong",false,false},
	}{
		info,err:=demoVerifier(context.Background(),tc.token,nil)
		if !tc.valid {
			if err==nil {t.Errorf("%q accepted",tc.token)}
			continue
		}
		if err!=nil || info==nil {t.Errorf("%q rejected: %v",tc.token,err);continue}
		hasWrite:=false
		for _,scope:=range info.Scopes {if scope=="showcase:write" {hasWrite=true}}
		if hasWrite!=tc.write {t.Errorf("%q write=%t, want %t",tc.token,hasWrite,tc.write)}
	}
}
