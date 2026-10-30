package main

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/airef"
	"github.com/dotdevlabs/ctlkit/pkg/root"
	"github.com/dotdevlabs/ctlkit/pkg/version"
	"github.com/spf13/cobra"

	"github.com/dotdevlabs/loopctl/internal/agents"
	"github.com/dotdevlabs/loopctl/internal/credentials"
	"github.com/dotdevlabs/loopctl/internal/environments"
	"github.com/dotdevlabs/loopctl/internal/onboard"
	"github.com/dotdevlabs/loopctl/internal/pipelines"
	"github.com/dotdevlabs/loopctl/internal/platforms"
	"github.com/dotdevlabs/loopctl/internal/projects"
	"github.com/dotdevlabs/loopctl/internal/schema"
	"github.com/dotdevlabs/loopctl/internal/taskkinds"
	"github.com/dotdevlabs/loopctl/internal/tasks"
	"github.com/dotdevlabs/loopctl/internal/topup"
)

func buildRoot() *cobra.Command {
	wfs := loopWorkflows()
	ver := version.Current("loopctl")
	cmd := root.New(root.BuildConfig{
		Product: "loopctl",
		Short:   "CLI for managing LoopControl",
		Version: ver,
		Commands: []*cobra.Command{
			agents.NewCmd(),
			credentials.NewCmd(),
			environments.NewCmd(),
			onboard.NewCmd(),
			platforms.NewCmd(),
			pipelines.NewCmd(),
			projects.NewCmd(),
			schema.NewCmd(),
			tasks.NewCmd(),
			taskkinds.NewCmd(),
			topup.NewCmd(),
		},
		Workflows: wfs,
	})
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
	for _, want := range []string{"onboard", "projects", "tasks", "task-kinds", "pipelines", "platforms", "schema"} {
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

// runAI executes the real root with the given args and returns stdout.
func runAI(t *testing.T, args ...string) string {
	t.Helper()
	cmd := buildRoot()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v failed: %v", args, err)
	}
	return out.String()
}

func findCommand(cmds []airef.CommandRef, use string) *airef.CommandRef {
	for i := range cmds {
		if strings.Fields(cmds[i].Use)[0] == use {
			return &cmds[i]
		}
	}
	return nil
}

func TestAIJSON(t *testing.T) {
	dec := json.NewDecoder(strings.NewReader(runAI(t, "ai", "--json")))
	dec.DisallowUnknownFields()
	var ref airef.Reference
	if err := dec.Decode(&ref); err != nil {
		t.Fatalf("ai --json is not an airef.Reference: %v", err)
	}
	if ref.Product != "loopctl" {
		t.Errorf("product = %q, want loopctl", ref.Product)
	}
	if len(ref.Commands) == 0 {
		t.Fatal("no commands in reference")
	}
	if !reflect.DeepEqual(ref.Workflows, loopWorkflows()) {
		t.Errorf("workflows differ from loopWorkflows()")
	}

	tasksRef := findCommand(ref.Commands, "tasks")
	if tasksRef == nil {
		t.Fatal("tasks command missing")
	}
	create := findCommand(tasksRef.Subcommands, "create")
	if create == nil {
		t.Fatal("tasks create missing")
	}
	required := map[string]bool{}
	for _, f := range create.Flags {
		required[f.Name] = f.Required
	}
	for _, name := range []string{"project-id", "kind", "title", "description"} {
		if !required[name] {
			t.Errorf("flag --%s should be marked required", name)
		}
	}
	if req, ok := required["depends-on"]; !ok || req {
		t.Errorf("flag --depends-on should exist and be optional (present=%v required=%v)", ok, req)
	}
}

func TestAIMarkdown(t *testing.T) {
	got := runAI(t, "ai")
	if json.Valid([]byte(got)) {
		t.Errorf("expected markdown, got JSON:\n%s", got)
	}
	wfs := loopWorkflows()
	for _, want := range []string{"# loopctl Command Reference", "## Common Workflows", wfs[0].Name} {
		if !strings.Contains(got, want) {
			t.Errorf("markdown missing %q; got:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "| yes |") {
		t.Errorf("markdown missing a required-flag row; got:\n%s", got)
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

// flagsByPath flattens the reference into "cmd sub sub" -> flag name -> FlagRef.
func flagsByPath(cmds []airef.CommandRef, prefix string, out map[string]map[string]airef.FlagRef) {
	for _, c := range cmds {
		path := strings.TrimSpace(prefix + " " + strings.Fields(c.Use)[0])
		m := map[string]airef.FlagRef{}
		for _, f := range c.Flags {
			m[f.Name] = f
		}
		out[path] = m
		flagsByPath(c.Subcommands, path, out)
	}
}

func aiFlags(t *testing.T) map[string]map[string]airef.FlagRef {
	t.Helper()
	var ref airef.Reference
	if err := json.Unmarshal([]byte(runAI(t, "ai", "--json")), &ref); err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]airef.FlagRef{}
	flagsByPath(ref.Commands, "", out)
	return out
}

func TestAIJSONFlagEnums(t *testing.T) {
	flags := aiFlags(t)
	todoStatus := []string{"pending", "in_progress", "completed"}
	cases := []struct {
		path, flag string
		want       []string
	}{
		{"tasks todos create", "status", todoStatus},
		{"tasks todos update", "status", todoStatus},
		{"pipelines stages add", "gate", []string{"automated", "human"}},
		{"pipelines stages update", "gate", []string{"automated", "human"}},
		{"pipelines stages add", "on-failure", []string{"rework", "block"}},
		{"pipelines stages update", "on-failure", []string{"rework", "block"}},
		{"environments entries add", "type", []string{"custom", "credential"}},
		{"environments entries change", "type", []string{"custom", "credential"}},
		{"topup", "product", []string{"trial", "topup", "subscription"}},
	}
	for _, c := range cases {
		f, ok := flags[c.path][c.flag]
		if !ok {
			t.Errorf("%s --%s missing", c.path, c.flag)
			continue
		}
		if !reflect.DeepEqual(f.Enum, c.want) {
			t.Errorf("%s --%s enum = %v, want %v", c.path, c.flag, f.Enum, c.want)
		}
	}
	// Open-ended, server-defined flags must not claim a fixed set.
	if f := flags["tasks create"]["kind"]; len(f.Enum) != 0 {
		t.Errorf("tasks create --kind enum = %v, want none", f.Enum)
	}
}

func TestAIJSONFlagSchemas(t *testing.T) {
	flags := aiFlags(t)
	cases := []struct{ path, flag, typ string }{
		{"pipelines create", "stages", "array"},
		{"pipelines update", "stages", "array"},
		{"pipelines stages add", "prompt-sections", "object"},
		{"pipelines stages update", "prompt-sections", "object"},
		{"pipelines stages add", "stage-triggers", "array"},
		{"pipelines stages update", "stage-triggers", "array"},
		{"pipelines stages add", "advance-requirements", "array"},
		{"pipelines stages update", "advance-requirements", "array"},
		{"tasks todos create", "bulk-json", "array"},
	}
	for _, c := range cases {
		f, ok := flags[c.path][c.flag]
		if !ok {
			t.Errorf("%s --%s missing", c.path, c.flag)
			continue
		}
		var s struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(f.Schema, &s); err != nil || s.Type != c.typ {
			t.Errorf("%s --%s schema = %s (type %q err %v), want type %q", c.path, c.flag, f.Schema, s.Type, err, c.typ)
		}
	}
}
