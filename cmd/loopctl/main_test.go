package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

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
		{"environments entries add", "type", []string{"credential", "custom"}},
		{"environments entries change", "type", []string{"credential", "custom"}},
		{"topup", "product", []string{"trial", "topup", "subscription_prepay"}},
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

// specNode walks the raw embedded spec YAML along a pointer, independently of
// the production helpers: first match wins on repeated keys, and a segment
// absent from a oneOf node is searched in its alternatives.
func specNode(n *yaml.Node, segs []string) []*yaml.Node {
	if len(segs) == 0 {
		return []*yaml.Node{n}
	}
	if n.Kind == yaml.DocumentNode {
		return specNode(n.Content[0], segs)
	}
	if n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == segs[0] {
			return specNode(n.Content[i+1], segs[1:])
		}
	}
	var out []*yaml.Node
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == "oneOf" {
			for _, alt := range n.Content[i+1].Content {
				out = append(out, specNode(alt, segs)...)
			}
		}
	}
	return out
}

func rawSpecEnum(t *testing.T, ptr string) []string {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal(schema.FixtureData(), &doc); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, n := range specNode(&doc, strings.Split(ptr, "/")) {
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == "enum" {
				for _, v := range n.Content[i+1].Content {
					out = append(out, v.Value)
				}
			}
		}
	}
	return out
}

// TestAIJSONFlagsMatchSpec fails when any flag enum differs from the spec copy.
func TestAIJSONFlagsMatchSpec(t *testing.T) {
	flags := aiFlags(t)
	c := "components/schemas/"
	cases := []struct{ path, flag, ptr string }{
		{"tasks todos create", "status", c + "TodoAttributes/properties/status"},
		{"tasks todos update", "status", c + "TodoAttributes/properties/status"},
		{"pipelines stages add", "gate", c + "PipelineStageInput/properties/gate"},
		{"pipelines stages update", "gate", c + "PipelineStageInput/properties/gate"},
		{"pipelines stages add", "on-failure", c + "PipelineStageInput/properties/on_failure"},
		{"pipelines stages update", "on-failure", c + "PipelineStageInput/properties/on_failure"},
		{"environments entries add", "type", c + "EnvironmentEntryAttributes/properties/entry_type"},
		{"environments entries change", "type", c + "EnvironmentEntryAttributes/properties/entry_type"},
		{"topup", "product", c + "FundingEnvelopeProduct/properties/kind"},
	}
	mapped := map[string]bool{}
	for _, tc := range cases {
		mapped[tc.path+" --"+tc.flag] = true
		want := rawSpecEnum(t, tc.ptr)
		if len(want) == 0 {
			t.Fatalf("%s: spec has no enum", tc.ptr)
		}
		if got := flags[tc.path][tc.flag].Enum; !reflect.DeepEqual(got, want) {
			t.Errorf("%s --%s enum = %v, spec %v", tc.path, tc.flag, got, want)
		}
	}
	for path, fm := range flags {
		for name, f := range fm {
			if len(f.Enum) > 0 && !mapped[path+" --"+name] {
				t.Errorf("%s --%s has an enum with no spec mapping", path, name)
			}
		}
	}
}

func TestAIJSONStageTriggersSchema(t *testing.T) {
	flags := aiFlags(t)
	for _, path := range []string{"pipelines stages add", "pipelines stages update"} {
		var s struct {
			Type  string `json:"type"`
			Items struct {
				Type                 string   `json:"type"`
				Required             []string `json:"required"`
				AdditionalProperties bool     `json:"additionalProperties"`
				Properties           map[string]struct {
					Type string   `json:"type"`
					Enum []string `json:"enum"`
				} `json:"properties"`
			} `json:"items"`
		}
		if err := json.Unmarshal(flags[path]["stage-triggers"].Schema, &s); err != nil {
			t.Fatal(err)
		}
		it := s.Items
		if s.Type != "array" || it.Type != "object" || it.AdditionalProperties {
			t.Errorf("%s: schema = %s", path, flags[path]["stage-triggers"].Schema)
		}
		if !reflect.DeepEqual(it.Required, []string{"timing", "handler", "config"}) {
			t.Errorf("%s: required = %v", path, it.Required)
		}
		if !reflect.DeepEqual(it.Properties["timing"].Enum, rawSpecEnum(t, "components/schemas/PipelineStageTrigger/properties/timing")) ||
			len(it.Properties["timing"].Enum) != 4 {
			t.Errorf("%s: timing enum = %v", path, it.Properties["timing"].Enum)
		}
		if it.Properties["handler"].Type != "string" || it.Properties["config"].Type != "object" {
			t.Errorf("%s: handler/config = %v", path, it.Properties)
		}
	}
}

// TestBaseURLEnvWithoutContext proves LOOPCTL_BASE_URL alone selects the server.
func TestBaseURLEnvWithoutContext(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("LOOPCTL_CONTEXT", "")
	t.Setenv("LOOPCTL_BASE_URL", srv.URL)
	t.Setenv("LOOPCTL_TOKEN", "tok")
	cmd := buildRoot()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"tasks", "todos", "list", "1", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if hits == 0 {
		t.Fatal("request did not reach LOOPCTL_BASE_URL server")
	}
}
