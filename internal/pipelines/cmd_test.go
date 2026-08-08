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

	"github.com/dotdevlabs/ctlkit/pkg/config"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

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
