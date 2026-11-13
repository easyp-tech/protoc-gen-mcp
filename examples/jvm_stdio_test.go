package examples_test

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestStandaloneJavaExampleOverStdio(t *testing.T) {
	runStandaloneJVMTodoExample(t, "6_java_standalone", "todo-java-standalone")
}

func TestStandaloneKotlinExampleOverStdio(t *testing.T) {
	runStandaloneJVMTodoExample(t, "7_kotlin_standalone", "todo-kotlin-standalone")
}

// runStandaloneJVMTodoExample builds a JVM example exactly as its README does
// (Gradle runs easyp and the local plugin) and drives the installed server.
func runStandaloneJVMTodoExample(t *testing.T, dir, project string) {
	t.Helper()
	for _, tool := range []string{"gradle", "java", "javac", "easyp"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not found in PATH; skipping JVM example", tool)
		}
		if tool == "java" || tool == "javac" {
			if out, err := exec.Command(tool, "-version").CombinedOutput(); err != nil {
				t.Skipf("%s is not usable (%v: %s); skipping JVM example", tool, err, out)
			}
		}
	}

	projectDir := filepath.Join(repoRoot(t), "examples", dir)
	build := exec.Command("gradle", "--no-daemon", "--quiet", "installDist")
	build.Dir = projectDir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("gradle installDist failed in %s: %v\n%s", projectDir, err, out)
	}

	cmd := exec.Command(filepath.Join(projectDir, "build", "install", project, "bin", project))
	cmd.Dir = projectDir
	client := newStdioClient(t, cmd)
	client.initialize()

	tools := client.listTools()
	for _, name := range []string{"todo_CreateTask", "todo_ListTasks", "todo_CompleteTask"} {
		findTool(t, tools.Tools, name)
	}

	created, rpcErr := client.callTool("todo_CreateTask", map[string]any{"title": "Ship JVM support", "labels": []any{"jvm"}})
	if rpcErr != nil || created.IsError {
		t.Fatalf("CreateTask failed: rpc=%v result=%+v", rpcErr, created)
	}
	assertTextStructuredContentMatch(t, "todo_CreateTask", created)
	task, ok := decodeMap(t, created.StructuredContent)["task"].(map[string]any)
	if !ok || task["title"] != "Ship JVM support" {
		t.Fatalf("created task = %v", decodeMap(t, created.StructuredContent))
	}

	completed, rpcErr := client.callTool("todo_CompleteTask", map[string]any{"id": task["id"]})
	if rpcErr != nil || completed.IsError {
		t.Fatalf("CompleteTask failed: rpc=%v result=%+v", rpcErr, completed)
	}
	if done, _ := decodeMap(t, completed.StructuredContent)["task"].(map[string]any); done["done"] != true {
		t.Fatalf("completed task = %v", decodeMap(t, completed.StructuredContent))
	}

	listed, rpcErr := client.callTool("todo_ListTasks", map[string]any{"includeDone": false})
	if rpcErr != nil || listed.IsError {
		t.Fatalf("ListTasks failed: rpc=%v result=%+v", rpcErr, listed)
	}
	if tasks, _ := decodeMap(t, listed.StructuredContent)["tasks"].([]any); len(tasks) != 0 {
		t.Fatalf("open tasks after completion = %v, want none", tasks)
	}
}
