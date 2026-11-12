package mcpruntime

import "testing"

func TestConfiguredServerPreservesSDKLoggingCapability(t *testing.T) {
	for _, apps := range []bool{false, true} {
		name := "without apps"
		if apps {
			name = "with apps"
		}
		t.Run(name, func(t *testing.T) {
			server, err := NewConfiguredServer("configured", "v1", apps)
			if err != nil {
				t.Fatal(err)
			}
			initialized := testSDKServerClient(t, server).InitializeResult()
			if initialized == nil || initialized.Capabilities == nil {
				t.Fatalf("missing initialized capabilities: %+v", initialized)
			}
			if initialized.Capabilities.Logging == nil {
				t.Fatal("configured server dropped the SDK's default logging capability")
			}
			_, hasApps := initialized.Capabilities.Extensions[AppExtensionID]
			if hasApps != apps {
				t.Fatalf("Apps advertised = %t, want %t", hasApps, apps)
			}
		})
	}
}

func TestConfiguredAppsServerStillInfersResourceCapability(t *testing.T) {
	server, err := NewConfiguredServer("configured", "v1", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := RegisterAppResource(server, AppResource{
		URI: "ui://example/app", Name: "App", HTML: "<html>App</html>",
	}); err != nil {
		t.Fatal(err)
	}
	capabilities := testSDKServerClient(t, server).InitializeResult().Capabilities
	if capabilities.Resources == nil || !capabilities.Resources.ListChanged {
		t.Fatalf("resource capabilities = %+v, want inferred listChanged", capabilities.Resources)
	}
	if capabilities.Logging == nil {
		t.Fatal("Apps resources disabled the SDK's logging capability")
	}
	// AppCapabilities remains an extension-only helper for callers who want to
	// choose their own SDK capabilities instead of using NewConfiguredServer.
	if AppCapabilities().Logging != nil {
		t.Fatal("AppCapabilities unexpectedly changed the caller's logging policy")
	}
}
