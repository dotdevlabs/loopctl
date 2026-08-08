package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/airef"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/httpclient"
	"github.com/dotdevlabs/ctlkit/pkg/output"
	"github.com/dotdevlabs/ctlkit/pkg/root"
	"github.com/dotdevlabs/ctlkit/pkg/version"
	"github.com/spf13/cobra"

	"github.com/dotdevlabs/loopctl/internal/pipelines"
	"github.com/dotdevlabs/loopctl/internal/platforms"
	"github.com/dotdevlabs/loopctl/internal/projects"
	"github.com/dotdevlabs/loopctl/internal/schema"
	"github.com/dotdevlabs/loopctl/internal/taskkinds"
	"github.com/dotdevlabs/loopctl/internal/tasks"
)

func buildRoot() *cobra.Command {
	wfs := loopWorkflows()
	ver := version.Current("loopctl")
	cmd := root.New(root.BuildConfig{
		Product: "loopctl",
		Short:   "CLI for managing LoopControl",
		Version: ver,
		Commands: []*cobra.Command{
			platforms.NewCmd(),
			pipelines.NewCmd(),
			projects.NewCmd(),
			schema.NewCmd(),
			tasks.NewCmd(),
			taskkinds.NewCmd(),
		},
		Workflows: wfs,
	})
	for _, sub := range cmd.Commands() {
		if sub.Name() == "ai" {
			cmd.RemoveCommand(sub)
			break
		}
	}
	cmd.AddCommand(newAICmd(ver, wfs))
	return cmd
}

func TestRootHasGlobalFlags(t *testing.T) {
	cmd := buildRoot()
	flags := []string{"json", "context", "dry-run", "format", "verbose"}
	for _, name := range flags {
		if cmd.PersistentFlags().Lookup(name) == nil {
			t.Errorf("missing persistent flag --%s", name)
		}
	}
}

func TestRootHasResourceCommands(t *testing.T) {
	cmd := buildRoot()
	names := map[string]bool{}
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"projects", "tasks", "task-kinds", "pipelines", "platforms", "schema"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestRootHasBuiltinCommands(t *testing.T) {
	cmd := buildRoot()
	names := map[string]bool{}
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"auth", "context", "version", "ai"} {
		if !names[want] {
			t.Errorf("missing builtin command %q", want)
		}
	}
}

func TestAICommandMarkdown(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	var out bytes.Buffer
	client := httpclient.NewWithTransport(ts.URL, "tok", http.DefaultTransport)
	renderer := output.New(false, "", &out, io.Discard)

	ctx := ctxutil.WithClient(context.Background(), client)
	ctx = ctxutil.WithRenderer(ctx, renderer)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{})

	ver := version.Current("loopctl")
	wfs := loopWorkflows()
	aiCmd := newAICmd(ver, wfs)
	aiCmd.SetContext(ctx)
	aiCmd.SetOut(&out)

	if err := aiCmd.RunE(aiCmd, nil); err != nil {
		t.Fatalf("ai cmd failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "loopctl Command Reference") {
		t.Errorf("expected markdown heading; got:\n%s", got)
	}
	if !strings.Contains(got, "Common Workflows") {
		t.Errorf("expected workflows section; got:\n%s", got)
	}
}

func TestAICommandJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	var out bytes.Buffer
	client := httpclient.NewWithTransport(ts.URL, "tok", http.DefaultTransport)
	renderer := output.New(true, "", &out, io.Discard)

	ctx := ctxutil.WithClient(context.Background(), client)
	ctx = ctxutil.WithRenderer(ctx, renderer)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{JSON: true})

	// Build root so the ai command has a parent with the --json flag.
	rootCmd := buildRoot()
	rootCmd.SetOut(&out)

	// Find the ai command.
	var aiCmd *cobra.Command
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "ai" {
			aiCmd = sub
			break
		}
	}
	if aiCmd == nil {
		t.Fatal("ai command not found")
	}
	aiCmd.SetContext(ctx)
	aiCmd.SetOut(&out)
	_ = renderer
	_ = client

	// Set --json on root's persistent flags.
	if err := rootCmd.PersistentFlags().Set("json", "true"); err != nil {
		t.Fatal(err)
	}

	if err := aiCmd.RunE(aiCmd, nil); err != nil {
		t.Fatalf("ai cmd failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"product"`) {
		t.Errorf("expected JSON with product field; got:\n%s", got)
	}
	if !strings.Contains(got, `"loopctl"`) {
		t.Errorf("expected loopctl in JSON; got:\n%s", got)
	}
}

func TestLoopWorkflows(t *testing.T) {
	wfs := loopWorkflows()
	if len(wfs) == 0 {
		t.Fatal("expected at least one workflow")
	}
	for _, wf := range wfs {
		if wf.Name == "" {
			t.Error("workflow missing name")
		}
		if len(wf.Steps) == 0 {
			t.Errorf("workflow %q has no steps", wf.Name)
		}
	}
}

func TestRenderAIMarkdown(t *testing.T) {
	ref := airef.Reference{
		Product:  "loopctl",
		Version:  "dev",
		Commands: []airef.CommandRef{{Use: "projects", Short: "Manage projects"}},
		Workflows: []airef.Workflow{
			{Name: "Test workflow", Description: "A test", Steps: []string{"step 1"}},
		},
	}
	var buf bytes.Buffer
	if err := renderAIMarkdown(&buf, ref); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "loopctl Command Reference") {
		t.Error("missing heading")
	}
	if !strings.Contains(out, "Test workflow") {
		t.Error("missing workflow name")
	}
}
