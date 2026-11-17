package pipelines

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/config"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

func TestCurrentStageInputs(t *testing.T) {
	stageJSON := `{"custom_stage_name":"analysis","stage_type":"custom","prompt_sections":{"template":"Analyze it."}}`
	for _, operation := range []string{"create", "update", "add", "stage-update"} {
		t.Run(operation, func(t *testing.T) {
			var captured []byte
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/vnd.api+json")
				if r.Method == http.MethodGet {
					_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"p1","attributes":{"name":"Pipeline","kind":"feature","stages":[{"stage_type":"existing","approval_button_text":"Keep label"}]}}}`)
					return
				}
				captured, _ = io.ReadAll(r.Body)
				if operation == "stage-update" {
					_, _ = fmt.Fprint(w, `{"data":{"type":"pipeline_stages","id":"s1","attributes":{"stage_type":"custom","custom_stage_name":"analysis","prompt_sections":{"template":"Analyze it."}}}}`)
				} else {
					_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"p1","attributes":{"name":"Pipeline","kind":"feature"}}}`)
				}
			}))
			defer ts.Close()
			var cmd *cobra.Command
			var args []string
			switch operation {
			case "create":
				cmd = createCmd()
				_ = cmd.Flags().Set("name", "Pipeline")
			case "update":
				cmd, args = updateCmd(), []string{"p1"}
			case "add":
				cmd, args = stagesAddCmd(), []string{"p1"}
			default:
				cmd, args = stagesUpdateCmd(), []string{"p1", "s1"}
			}
			if operation == "create" || operation == "update" {
				_ = cmd.Flags().Set("stages", "["+stageJSON+"]")
				if !strings.Contains(cmd.Flags().Lookup("stages").Usage, "custom_stage_name") {
					t.Fatal("help lacks current example")
				}
			} else {
				_ = cmd.Flags().Set("custom-stage-name", "analysis")
				_ = cmd.Flags().Set("stage-type", "custom")
				_ = cmd.Flags().Set("prompt-sections", `{"template":"Analyze it."}`)
				if cmd.Flags().Lookup("role").Deprecated != "Deprecated: use stage_type: 'custom'." {
					t.Fatal("role lacks contract deprecation")
				}
				if cmd.Flags().Lookup("stage-type").Usage != "Server stage type name" {
					t.Fatal("stage type help must be generic")
				}
			}
			var out bytes.Buffer
			if operation == "add" {
				if err := cmd.ValidateRequiredFlags(); err != nil {
					t.Fatalf("current fields must not require legacy name: %v", err)
				}
			}
			cmd.SetOut(&out)
			cmd.SetContext(makeCtx(t, ts.URL, "tok", true, &out))
			if err := cmd.RunE(cmd, args); err != nil {
				t.Fatal(err)
			}
			var body map[string]json.RawMessage
			if err := json.Unmarshal(captured, &body); err != nil {
				t.Fatal(err)
			}
			var stage map[string]any
			if operation == "stage-update" {
				if err := json.Unmarshal(body["pipeline_stage"], &stage); err != nil {
					t.Fatal(err)
				}
			} else {
				var pipeline struct {
					Stages []map[string]any `json:"stages"`
				}
				if err := json.Unmarshal(body["pipeline"], &pipeline); err != nil {
					t.Fatal(err)
				}
				if operation == "add" && (len(pipeline.Stages) != 2 || pipeline.Stages[0]["stage_type"] != "existing" || pipeline.Stages[0]["approval_button_text"] != "Keep label") {
					t.Fatalf("existing stage lost: %s", captured)
				}
				stage = pipeline.Stages[len(pipeline.Stages)-1]
			}
			if len(stage) != 3 || stage["custom_stage_name"] != "analysis" || stage["stage_type"] != "custom" || stage["prompt_sections"].(map[string]any)["template"] != "Analyze it." {
				t.Fatalf("unexpected stage: %s", captured)
			}
		})
	}
}

func TestApprovalButtonTextRequests(t *testing.T) {
	cases := []struct {
		name    string
		command func() *cobra.Command
		args    []string
		request string
		prefix  string
		suffix  string
	}{
		{"create", createCmd, nil, "POST /api/pipelines", `{"pipeline":{"kind":"","name":"Delivery","stages":[{`, `}]}}`},
		{"update", updateCmd, []string{"p1"}, "PATCH /api/pipelines/p1", `{"pipeline":{"stages":[{`, `}]}}`},
		{"add", stagesAddCmd, []string{"p1"}, "PATCH /api/pipelines/p1", `{"pipeline":{"stages":[{`, `}]}}`},
		{"stage-update", stagesUpdateCmd, []string{"p1", "s1"}, "PATCH /api/pipelines/p1/stages/s1", `{"pipeline_stage":{`, `}}`},
	}
	for _, tc := range cases {
		for _, label := range []string{" Approve release ", ""} {
			for _, dryRun := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%q/dry-run=%t", tc.name, label, dryRun), func(t *testing.T) {
					property := fmt.Sprintf(`"approval_button_text":%q`, label)
					want := tc.prefix + property + tc.suffix
					var captured string
					ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if dryRun {
							t.Error("preview made an HTTP request")
						}
						w.Header().Set("Content-Type", "application/vnd.api+json")
						if tc.name == "add" && r.Method == http.MethodGet {
							_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"p1","attributes":{"stages":[]}}}`)
							return
						}
						if got := r.Method + " " + r.URL.Path; got != tc.request {
							t.Errorf("request = %q, want %q", got, tc.request)
						}
						body, err := io.ReadAll(r.Body)
						if err != nil {
							t.Error(err)
						}
						captured = string(bytes.TrimSpace(body))
						if tc.name == "stage-update" {
							_, _ = fmt.Fprint(w, `{"data":{"type":"pipeline_stages","id":"s1","attributes":{}}}`)
							return
						}
						_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"p1","attributes":{}}}`)
					}))
					defer ts.Close()
					cmd := tc.command()
					if tc.name == "create" {
						_ = cmd.Flags().Set("name", "Delivery")
					}
					switch tc.name {
					case "create", "update":
						_ = cmd.Flags().Set("stages", "[{"+property+"}]")
					default:
						_ = cmd.Flags().Set("approval-button-text", label)
					}
					var out bytes.Buffer
					ctx := makeCtx(t, ts.URL, "tok", false, &out)
					cmd.SetContext(ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{DryRun: dryRun}))
					cmd.SetOut(&out)
					if err := cmd.RunE(cmd, tc.args); err != nil {
						t.Fatal(err)
					}
					if dryRun {
						request := tc.request
						if tc.name == "add" {
							request += " (stage append fragment)"
						}
						if got := strings.TrimSpace(out.String()); got != "dry-run: would "+request+" "+want {
							t.Errorf("unexpected preview: %s", got)
						}
					} else if captured != want {
						t.Errorf("request body = %s, want %s", captured, want)
					}
				})
			}
		}
	}
}

func TestStagesRejectNonObjectArrays(t *testing.T) {
	for _, makeCmd := range []func() *cobra.Command{createCmd, updateCmd} {
		for _, input := range []string{`null`, `[null]`, `{}`, `["stage"]`, `invalid`} {
			cmd := makeCmd()
			_ = cmd.Flags().Set("stages", input)
			cmd.SetContext(makeCtx(t, "http://unused.invalid", "tok", true, io.Discard))
			if err := cmd.RunE(cmd, []string{"p1"}); err == nil || !strings.Contains(err.Error(), "--stages") {
				t.Fatalf("expected stages error for %s: %v", input, err)
			}
		}
	}
}

func TestStageHelpUsesCurrentFields(t *testing.T) {
	for _, cmd := range []*cobra.Command{createCmd(), updateCmd(), stagesAddCmd(), stagesUpdateCmd()} {
		var out bytes.Buffer
		cmd.SetOut(&out)
		if err := cmd.Help(); err != nil {
			t.Fatal(err)
		}
		for _, obsolete := range []string{"planning, implementing", `"role":"planning"`, "Stage name (required)", "stage_type for custom stage types"} {
			if strings.Contains(out.String(), obsolete) {
				t.Fatalf("obsolete help %q: %s", obsolete, &out)
			}
		}
	}
}

func makeCtx(t *testing.T, serverURL, token string, jsonMode bool, out io.Writer) context.Context {
	t.Helper()
	renderer := output.New(jsonMode, "", out, io.Discard)
	ctx := context.Background()
	ctx = ctxutil.WithRenderer(ctx, renderer)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{JSON: jsonMode})
	ctx = ctxutil.WithActiveContext(ctx, &config.Context{BaseURL: serverURL, Token: token})
	return ctx
}

func TestPipelinesList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pipelines" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing/wrong auth header: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"pipelines","id":"pipe1","attributes":{"name":"autonomous-feature","kind":"my-task-kind"}}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "test-token", false, &out)

	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("list failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "pipe1") {
		t.Errorf("output missing pipeline id:\n%s", got)
	}
	if !strings.Contains(got, "autonomous-feature") {
		t.Errorf("output missing pipeline name:\n%s", got)
	}
	if !strings.Contains(got, "my-task-kind") {
		t.Errorf("output missing kind:\n%s", got)
	}
}

func TestPipelinesListJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"pipelines","id":"pipe1","attributes":{"name":"autonomous-feature","kind":"my-task-kind"}}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", true, &out)

	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("list json failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"data"`) {
		t.Errorf("expected JSON collection envelope; got:\n%s", got)
	}
	if !strings.Contains(got, "autonomous-feature") {
		t.Errorf("expected pipeline name in JSON; got:\n%s", got)
	}
}

func TestPipelinesListError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"500","detail":"internal server error"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error on 500")
	}
	if !strings.Contains(err.Error(), "internal server error") {
		t.Errorf("expected error detail in message; got: %v", err)
	}
}

func TestPipelinesListVerbose(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"pipelines","id":"pipe1","attributes":{"name":"autonomous-feature","kind":"my-task-kind"}}]}`)
	}))
	defer ts.Close()

	var out, errBuf bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	ctx = apiclient.WithVerbose(ctx, &errBuf)

	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("list verbose failed: %v", err)
	}

	verbose := errBuf.String()
	if !strings.Contains(verbose, "GET") {
		t.Errorf("expected GET in verbose output; got: %q", verbose)
	}
	if !strings.Contains(verbose, "/api/pipelines") {
		t.Errorf("expected /api/pipelines in verbose output; got: %q", verbose)
	}
	if !strings.Contains(verbose, "200") {
		t.Errorf("expected status 200 in verbose output; got: %q", verbose)
	}
}

func TestPipelinesCreate(t *testing.T) {
	var gotBody []byte
	var gotContentType string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pipelines" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"pipe-new","attributes":{"name":"custom-pipeline","kind":"my-task-kind"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "test-token", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "custom-pipeline"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("kind", "my-task-kind"); err != nil {
		t.Fatal(err)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "pipe-new") {
		t.Errorf("output missing created id:\n%s", got)
	}
	if !strings.Contains(string(gotBody), `"pipeline"`) {
		t.Errorf("request body missing pipeline wrapper: %s", gotBody)
	}
	if !strings.Contains(string(gotBody), `"name"`) {
		t.Errorf("request body missing name: %s", gotBody)
	}
	if !strings.Contains(string(gotBody), `"kind"`) {
		t.Errorf("request body missing kind: %s", gotBody)
	}
	if strings.Contains(string(gotBody), "display_name") {
		t.Errorf("request body must not contain display_name: %s", gotBody)
	}
	if strings.Contains(string(gotBody), "stages") {
		t.Errorf("request body must not contain stages: %s", gotBody)
	}
	if gotContentType != "application/json" {
		t.Errorf("expected Content-Type application/json; got: %s", gotContentType)
	}
}

func TestPipelinesCreateWithoutKind(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"pipe-nokind","attributes":{"name":"no-kind-pipeline","kind":""}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "no-kind-pipeline"); err != nil {
		t.Fatal(err)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create without kind failed: %v", err)
	}
	if !strings.Contains(out.String(), "pipe-nokind") {
		t.Errorf("expected created id in output; got: %s", out.String())
	}
}

func TestPipelinesCreateJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"pipe-new","attributes":{"name":"custom-pipeline","kind":"my-task-kind"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", true, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "custom-pipeline"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("kind", "my-task-kind"); err != nil {
		t.Fatal(err)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create json failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"id"`) {
		t.Errorf("expected id in JSON output; got:\n%s", got)
	}
	if !strings.Contains(got, "pipe-new") {
		t.Errorf("expected id value in JSON output; got:\n%s", got)
	}
}

func TestPipelinesCreateDryRun(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{DryRun: true})

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "my-pipeline"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("kind", "my-task-kind"); err != nil {
		t.Fatal(err)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create dry-run failed: %v", err)
	}
	if called {
		t.Error("dry-run should not make HTTP calls")
	}
	if !strings.Contains(out.String(), "dry-run") {
		t.Errorf("expected dry-run in output; got:\n%s", out.String())
	}
}

func TestPipelinesCreateError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"422","detail":"name can't be blank"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "bad-pipeline"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("kind", "my-task-kind"); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error on 422")
	}
	if !strings.Contains(err.Error(), "name can't be blank") {
		t.Errorf("expected detail in error; got: %v", err)
	}
}

func TestPipelinesCreateBuiltInRejected(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"403","detail":"built-in pipeline cannot be modified"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "built-in-pipeline"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("kind", "my-task-kind"); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for built-in rejection")
	}
	if !strings.Contains(err.Error(), "built-in pipeline cannot be modified") {
		t.Errorf("expected built-in error message; got: %v", err)
	}
}

func TestPipelinesCreateVerbose(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"pipe-new","attributes":{"name":"custom-pipeline","kind":"my-task-kind"}}}`)
	}))
	defer ts.Close()

	var out, errBuf bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	ctx = apiclient.WithVerbose(ctx, &errBuf)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "custom-pipeline"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("kind", "my-task-kind"); err != nil {
		t.Fatal(err)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create verbose failed: %v", err)
	}

	verbose := errBuf.String()
	if !strings.Contains(verbose, "POST") {
		t.Errorf("expected POST in verbose output; got: %q", verbose)
	}
	if !strings.Contains(verbose, "/api/pipelines") {
		t.Errorf("expected /api/pipelines in verbose output; got: %q", verbose)
	}
	if !strings.Contains(verbose, "201") && !strings.Contains(verbose, "200") {
		t.Errorf("expected 2xx status in verbose output; got: %q", verbose)
	}
}

// TestCreatePipelineRequestBodyShape asserts the create-pipeline request body
// is a "pipeline" object containing "name" and "kind", and no "stages".
func TestCreatePipelineRequestBodyShape(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"pipe-new","attributes":{"name":"my-pipeline","kind":"my-task-kind"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "my-pipeline"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("kind", "my-task-kind"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(gotBody, &parsed); err != nil {
		t.Fatalf("request body is not valid JSON: %v\nbody: %s", err, gotBody)
	}

	rawPipeline, ok := parsed["pipeline"]
	if !ok {
		t.Fatalf("request body missing top-level 'pipeline' key; got keys: %v\nbody: %s", mapKeys(parsed), gotBody)
	}

	var pipeObj map[string]json.RawMessage
	if err := json.Unmarshal(rawPipeline, &pipeObj); err != nil {
		t.Fatalf("pipeline value is not a JSON object: %v", err)
	}

	if _, ok := pipeObj["name"]; !ok {
		t.Errorf("pipeline object missing 'name' field; got keys: %v", mapKeys(pipeObj))
	}
	if _, ok := pipeObj["kind"]; !ok {
		t.Errorf("pipeline object missing 'kind' field; got keys: %v", mapKeys(pipeObj))
	}
	if _, ok := pipeObj["stages"]; ok {
		t.Errorf("pipeline object must not contain 'stages' field")
	}
	if _, ok := pipeObj["display_name"]; ok {
		t.Errorf("pipeline object must not contain 'display_name' field")
	}
}

// TestCreatePipelineRequestBodyWithDescription asserts the optional description
// field is included in the request body when provided.
func TestCreatePipelineRequestBodyWithDescription(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"pipe-new","attributes":{"name":"my-pipeline","kind":"my-task-kind","description":"Some text"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "my-pipeline"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("kind", "my-task-kind"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("description", "Some text"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(gotBody, &parsed); err != nil {
		t.Fatalf("request body is not valid JSON: %v\nbody: %s", err, gotBody)
	}

	rawPipeline, ok := parsed["pipeline"]
	if !ok {
		t.Fatalf("request body missing top-level 'pipeline' key; body: %s", gotBody)
	}

	var pipeObj map[string]json.RawMessage
	if err := json.Unmarshal(rawPipeline, &pipeObj); err != nil {
		t.Fatalf("pipeline value is not a JSON object: %v", err)
	}

	if _, ok := pipeObj["description"]; !ok {
		t.Errorf("pipeline object missing 'description' field when --description provided; got keys: %v", mapKeys(pipeObj))
	}
	if _, ok := pipeObj["stages"]; ok {
		t.Errorf("pipeline object must not contain 'stages' field")
	}
}

func mapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestPipelinesCreateWithStages(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"p1","attributes":{"name":"my-pipeline","kind":""}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "my-pipeline"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("stages", `[{"name":"plan","role":"working","instructions":"Plan the work."}]`); err != nil {
		t.Fatal(err)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create with stages failed: %v", err)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(gotBody, &parsed); err != nil {
		t.Fatalf("request body is not valid JSON: %v\nbody: %s", err, gotBody)
	}

	rawPipeline, ok := parsed["pipeline"]
	if !ok {
		t.Fatalf("request body missing 'pipeline' key; body: %s", gotBody)
	}

	var pipeObj map[string]json.RawMessage
	if err := json.Unmarshal(rawPipeline, &pipeObj); err != nil {
		t.Fatalf("pipeline value is not a JSON object: %v", err)
	}

	rawStages, ok := pipeObj["stages"]
	if !ok {
		t.Fatalf("pipeline object missing 'stages' field; got keys: %v", mapKeys(pipeObj))
	}

	var stages []map[string]json.RawMessage
	if err := json.Unmarshal(rawStages, &stages); err != nil {
		t.Fatalf("stages is not a JSON array of objects: %v", err)
	}
	if len(stages) != 1 {
		t.Fatalf("expected 1 stage; got: %d", len(stages))
	}
	for _, key := range []string{"name", "role", "instructions"} {
		if _, ok := stages[0][key]; !ok {
			t.Errorf("stage missing %q field; got keys: %v", key, mapKeys(stages[0]))
		}
	}
}

func TestPipelinesCreateWithStages_BadJSON(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "my-pipeline"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("stages", `not-valid-json`); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid --stages JSON")
	}
	if called {
		t.Error("HTTP call should not be made when --stages JSON is invalid")
	}
}

func TestPipelinesUpdate(t *testing.T) {
	var gotBody []byte
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"pipe1","attributes":{"name":"updated-name","kind":""}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "updated-name"); err != nil {
		t.Fatal(err)
	}

	if err := cmd.RunE(cmd, []string{"pipe1"}); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if gotMethod != http.MethodPatch {
		t.Errorf("expected PATCH; got: %s", gotMethod)
	}
	if gotPath != "/api/pipelines/pipe1" {
		t.Errorf("expected /api/pipelines/pipe1; got: %s", gotPath)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(gotBody, &parsed); err != nil {
		t.Fatalf("request body is not valid JSON: %v", err)
	}
	if _, ok := parsed["pipeline"]; !ok {
		t.Fatalf("request body missing 'pipeline' key; body: %s", gotBody)
	}

	got := out.String()
	if !strings.Contains(got, "pipe1") {
		t.Errorf("output missing updated id:\n%s", got)
	}
}

func TestPipelinesUpdateWithStages(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"p1","attributes":{"name":"my-pipeline","kind":""}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("stages", `[{"custom_stage_name":"plan","prompt_sections":{"template":"Plan."}},{"custom_stage_name":"implement","prompt_sections":{"template":"Implement."}}]`); err != nil {
		t.Fatal(err)
	}

	if err := cmd.RunE(cmd, []string{"p1"}); err != nil {
		t.Fatalf("update with stages failed: %v", err)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(gotBody, &parsed); err != nil {
		t.Fatalf("request body is not valid JSON: %v", err)
	}
	rawPipeline := parsed["pipeline"]
	var pipeObj map[string]json.RawMessage
	if err := json.Unmarshal(rawPipeline, &pipeObj); err != nil {
		t.Fatalf("pipeline value is not a JSON object: %v", err)
	}
	rawStages, ok := pipeObj["stages"]
	if !ok {
		t.Fatalf("pipeline object missing 'stages' field; got keys: %v", mapKeys(pipeObj))
	}
	var stages []map[string]json.RawMessage
	if err := json.Unmarshal(rawStages, &stages); err != nil {
		t.Fatalf("stages is not a JSON array of objects: %v", err)
	}
	if len(stages) != 2 {
		t.Fatalf("expected 2 stages; got: %d", len(stages))
	}
}

func TestPipelinesUpdateDryRun(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{DryRun: true})

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "new-name"); err != nil {
		t.Fatal(err)
	}

	if err := cmd.RunE(cmd, []string{"pipe1"}); err != nil {
		t.Fatalf("update dry-run failed: %v", err)
	}
	if called {
		t.Error("dry-run should not make HTTP calls")
	}
	if !strings.Contains(out.String(), "dry-run") {
		t.Errorf("expected dry-run in output; got:\n%s", out.String())
	}
}

func TestPipelinesUpdateError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"404","detail":"pipeline not found"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "updated"); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, []string{"missing-id"})
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if !strings.Contains(err.Error(), "pipeline not found") {
		t.Errorf("expected 'pipeline not found' in error; got: %v", err)
	}
}

func TestPipelinesUpdateNoFlags(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{"pipe1"})
	if err == nil {
		t.Fatal("expected error when no flags provided")
	}
	if called {
		t.Error("HTTP call should not be made when no flags provided")
	}
}

func TestPipelinesUpdateOnlyChangedFlags(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"p1","attributes":{"name":"x","kind":""}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("kind", "my-kind"); err != nil {
		t.Fatal(err)
	}

	if err := cmd.RunE(cmd, []string{"p1"}); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(gotBody, &parsed); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	rawPipeline := parsed["pipeline"]
	var pipeObj map[string]json.RawMessage
	if err := json.Unmarshal(rawPipeline, &pipeObj); err != nil {
		t.Fatalf("pipeline value not a JSON object: %v", err)
	}
	// Only "kind" should be present, not "name" (which was not changed).
	if _, ok := pipeObj["name"]; ok {
		t.Errorf("pipeline object should not contain 'name' when --name not set; got keys: %v", mapKeys(pipeObj))
	}
	if _, ok := pipeObj["kind"]; !ok {
		t.Errorf("pipeline object missing 'kind' field; got keys: %v", mapKeys(pipeObj))
	}
}
