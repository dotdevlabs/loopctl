package taskkinds

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

func TestTaskKindsList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/task_kinds" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing/wrong auth header: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"task_kinds","id":"kind1","attributes":{"name":"feature","built_in":true}}]}`)
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
	if !strings.Contains(got, "kind1") {
		t.Errorf("output missing kind id:\n%s", got)
	}
	if !strings.Contains(got, "feature") {
		t.Errorf("output missing kind name:\n%s", got)
	}
	if !strings.Contains(got, "true") {
		t.Errorf("output missing built_in:\n%s", got)
	}
}

func TestTaskKindsListJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"task_kinds","id":"kind1","attributes":{"name":"feature","built_in":true}}]}`)
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
	if !strings.Contains(got, "feature") {
		t.Errorf("expected kind name in JSON; got:\n%s", got)
	}
}

func TestTaskKindsListError(t *testing.T) {
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

func TestTaskKindsListVerbose(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"task_kinds","id":"kind1","attributes":{"name":"feature","built_in":false}}]}`)
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
	if !strings.Contains(verbose, "/api/task_kinds") {
		t.Errorf("expected /api/task_kinds in verbose output; got: %q", verbose)
	}
	if !strings.Contains(verbose, "200") {
		t.Errorf("expected status 200 in verbose output; got: %q", verbose)
	}
}

func TestTaskKindsCreate(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/task_kinds" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"task_kinds","id":"kind-new","attributes":{"name":"custom-kind","built_in":false}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "test-token", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "custom-kind"); err != nil {
		t.Fatal(err)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "kind-new") {
		t.Errorf("output missing created id:\n%s", got)
	}
	if !strings.Contains(string(gotBody), `"task_kind"`) {
		t.Errorf("request body missing task_kind wrapper: %s", gotBody)
	}
	if !strings.Contains(string(gotBody), "custom-kind") {
		t.Errorf("request body missing name: %s", gotBody)
	}
	if strings.Contains(string(gotBody), "display_name") {
		t.Errorf("request body must not contain display_name: %s", gotBody)
	}
}

func TestTaskKindsCreateJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"task_kinds","id":"kind-new","attributes":{"name":"custom-kind","built_in":false}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", true, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "custom-kind"); err != nil {
		t.Fatal(err)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create json failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"id"`) {
		t.Errorf("expected id in JSON output; got:\n%s", got)
	}
	if !strings.Contains(got, "kind-new") {
		t.Errorf("expected id value in JSON output; got:\n%s", got)
	}
}

func TestTaskKindsCreateDryRun(t *testing.T) {
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
	if err := cmd.Flags().Set("name", "my-kind"); err != nil {
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

func TestTaskKindsCreateError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"422","detail":"name has already been taken"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "existing-kind"); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error on 422")
	}
	if !strings.Contains(err.Error(), "name has already been taken") {
		t.Errorf("expected detail in error; got: %v", err)
	}
}

func TestTaskKindsCreateBuiltInRejected(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"403","detail":"built-in task kind cannot be modified"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "feature"); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for built-in rejection")
	}
	if !strings.Contains(err.Error(), "built-in task kind cannot be modified") {
		t.Errorf("expected built-in error message; got: %v", err)
	}
}

// TestCreateTaskKindRequestBodyShape asserts that the request body sent to
// create a task kind is a "task_kind" object containing only "name".
func TestCreateTaskKindRequestBodyShape(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"task_kinds","id":"kind-new","attributes":{"name":"my-kind","built_in":false}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "my-kind"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(gotBody, &parsed); err != nil {
		t.Fatalf("request body is not valid JSON: %v\nbody: %s", err, gotBody)
	}

	rawKind, ok := parsed["task_kind"]
	if !ok {
		t.Fatalf("request body missing top-level 'task_kind' key; got keys: %v\nbody: %s", mapKeys(parsed), gotBody)
	}

	var kindObj map[string]json.RawMessage
	if err := json.Unmarshal(rawKind, &kindObj); err != nil {
		t.Fatalf("task_kind value is not a JSON object: %v", err)
	}

	if _, ok := kindObj["name"]; !ok {
		t.Errorf("task_kind object missing 'name' field; got keys: %v", mapKeys(kindObj))
	}

	for k := range kindObj {
		if k != "name" {
			t.Errorf("task_kind object contains unexpected field %q; only 'name' is allowed", k)
		}
	}
}

func mapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestTaskKindsSetDefaultPipeline verifies the new endpoint: PATCH /api/account_pipeline_defaults/{kind}.
func TestTaskKindsSetDefaultPipeline(t *testing.T) {
	var gotBody []byte
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"account_pipeline_defaults","id":"apd1","attributes":{"kind":"feature","pipeline_id":"123"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "test-token", false, &out)

	cmd := setDefaultPipelineCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("pipeline-id", "123"); err != nil {
		t.Fatal(err)
	}

	if err := cmd.RunE(cmd, []string{"feature"}); err != nil {
		t.Fatalf("set-default-pipeline failed: %v", err)
	}

	if gotMethod != http.MethodPatch {
		t.Errorf("expected PATCH; got %s", gotMethod)
	}
	if gotPath != "/api/account_pipeline_defaults/feature" {
		t.Errorf("expected path /api/account_pipeline_defaults/feature; got %s", gotPath)
	}
	got := out.String()
	if !strings.Contains(got, "apd1") {
		t.Errorf("output missing id:\n%s", got)
	}
	if !strings.Contains(string(gotBody), `"account_pipeline_default"`) {
		t.Errorf("request body missing account_pipeline_default wrapper: %s", gotBody)
	}
	if !strings.Contains(string(gotBody), `"pipeline_id"`) {
		t.Errorf("request body missing pipeline_id: %s", gotBody)
	}
}

func TestTaskKindsSetDefaultPipelineJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"account_pipeline_defaults","id":"apd1","attributes":{"kind":"feature","pipeline_id":"123"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", true, &out)

	cmd := setDefaultPipelineCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("pipeline-id", "123"); err != nil {
		t.Fatal(err)
	}

	if err := cmd.RunE(cmd, []string{"feature"}); err != nil {
		t.Fatalf("set-default-pipeline JSON failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"id"`) {
		t.Errorf("expected id in JSON output; got:\n%s", got)
	}
	if !strings.Contains(got, "apd1") {
		t.Errorf("expected id value in JSON output; got:\n%s", got)
	}
}

func TestTaskKindsSetDefaultPipelineDryRun(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{DryRun: true})

	cmd := setDefaultPipelineCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("pipeline-id", "123"); err != nil {
		t.Fatal(err)
	}

	if err := cmd.RunE(cmd, []string{"feature"}); err != nil {
		t.Fatalf("set-default-pipeline dry-run failed: %v", err)
	}
	if called {
		t.Error("dry-run should not make HTTP calls")
	}
	if !strings.Contains(out.String(), "dry-run") {
		t.Errorf("expected dry-run in output; got:\n%s", out.String())
	}
}

func TestTaskKindsSetDefaultPipelineInvalidID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := setDefaultPipelineCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("pipeline-id", "not-a-number"); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, []string{"feature"})
	if err == nil {
		t.Fatal("expected error for non-integer pipeline-id")
	}
	if !strings.Contains(err.Error(), "integer") {
		t.Errorf("expected integer error message; got: %v", err)
	}
}

func TestTaskKindsSetDefaultPipelineError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"422","detail":"pipeline not found"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := setDefaultPipelineCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("pipeline-id", "999"); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, []string{"feature"})
	if err == nil {
		t.Fatal("expected error on 422")
	}
	if !strings.Contains(err.Error(), "pipeline not found") {
		t.Errorf("expected error detail in message; got: %v", err)
	}
}

// TestSetDefaultPipelineRequestBodyShape verifies PATCH /api/account_pipeline_defaults/{kind}
// sends {"account_pipeline_default": {"pipeline_id": <integer>}}.
func TestSetDefaultPipelineRequestBodyShape(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"account_pipeline_defaults","id":"apd1","attributes":{"kind":"feature","pipeline_id":"123"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := setDefaultPipelineCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("pipeline-id", "123"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, []string{"feature"}); err != nil {
		t.Fatalf("set-default-pipeline failed: %v", err)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(gotBody, &parsed); err != nil {
		t.Fatalf("request body is not valid JSON: %v\nbody: %s", err, gotBody)
	}

	rawAPD, ok := parsed["account_pipeline_default"]
	if !ok {
		t.Fatalf("request body missing top-level 'account_pipeline_default' key; got keys: %v\nbody: %s", mapKeys(parsed), gotBody)
	}

	var apdObj map[string]json.RawMessage
	if err := json.Unmarshal(rawAPD, &apdObj); err != nil {
		t.Fatalf("account_pipeline_default value is not a JSON object: %v", err)
	}

	if _, ok := apdObj["pipeline_id"]; !ok {
		t.Errorf("account_pipeline_default object missing 'pipeline_id' field; got keys: %v", mapKeys(apdObj))
	}

	for k := range apdObj {
		if k != "pipeline_id" {
			t.Errorf("account_pipeline_default object contains unexpected field %q; only 'pipeline_id' is allowed", k)
		}
	}

	var pipelineID int64
	if err := json.Unmarshal(apdObj["pipeline_id"], &pipelineID); err != nil {
		t.Fatalf("pipeline_id should be an integer; got: %s", apdObj["pipeline_id"])
	}
	if pipelineID != 123 {
		t.Errorf("expected pipeline_id=123; got: %d", pipelineID)
	}
}

// TestTaskKindsClearDefaultPipeline verifies the new endpoint: DELETE /api/account_pipeline_defaults/{kind}.
func TestTaskKindsClearDefaultPipeline(t *testing.T) {
	var gotMethod, gotPath string
	var gotBodyLen int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		gotBodyLen = len(body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "test-token", false, &out)

	cmd := clearDefaultPipelineCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"feature"}); err != nil {
		t.Fatalf("clear-default-pipeline failed: %v", err)
	}

	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE; got %s", gotMethod)
	}
	if gotPath != "/api/account_pipeline_defaults/feature" {
		t.Errorf("expected path /api/account_pipeline_defaults/feature; got %s", gotPath)
	}
	if gotBodyLen != 0 {
		t.Errorf("expected empty body for DELETE; got %d bytes", gotBodyLen)
	}
	if !strings.Contains(out.String(), "cleared") {
		t.Errorf("expected 'cleared' in output; got:\n%s", out.String())
	}
}

func TestTaskKindsClearDefaultPipelineDryRun(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{DryRun: true})

	cmd := clearDefaultPipelineCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"feature"}); err != nil {
		t.Fatalf("clear-default-pipeline dry-run failed: %v", err)
	}
	if called {
		t.Error("dry-run should not make HTTP calls")
	}
	if !strings.Contains(out.String(), "dry-run") {
		t.Errorf("expected dry-run in output; got:\n%s", out.String())
	}
}

func TestTaskKindsClearDefaultPipelineError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"404","detail":"task kind not found"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := clearDefaultPipelineCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{"unknown-kind"})
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if !strings.Contains(err.Error(), "task kind not found") {
		t.Errorf("expected error detail in message; got: %v", err)
	}
}

func TestListDefaultPipelines(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/account_pipeline_defaults" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"account_pipeline_defaults","id":"apd1","attributes":{"kind":"feature","pipeline_id":"42"}}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "test-token", false, &out)

	cmd := listDefaultPipelinesCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("list-default-pipelines failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "apd1") {
		t.Errorf("output missing id:\n%s", got)
	}
	if !strings.Contains(got, "feature") {
		t.Errorf("output missing kind:\n%s", got)
	}
	if !strings.Contains(got, "42") {
		t.Errorf("output missing pipeline_id:\n%s", got)
	}
}

func TestListDefaultPipelinesJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"account_pipeline_defaults","id":"apd1","attributes":{"kind":"feature","pipeline_id":"42"}}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", true, &out)

	cmd := listDefaultPipelinesCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("list-default-pipelines JSON failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"data"`) {
		t.Errorf("expected JSON collection envelope; got:\n%s", got)
	}
	if !strings.Contains(got, "feature") {
		t.Errorf("expected kind in JSON output; got:\n%s", got)
	}
}

func TestListDefaultPipelinesError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"500","detail":"internal server error"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := listDefaultPipelinesCmd()
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

func TestListDefaultPipelinesVerbose(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"account_pipeline_defaults","id":"apd1","attributes":{"kind":"feature","pipeline_id":"42"}}]}`)
	}))
	defer ts.Close()

	var out, errBuf bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	ctx = apiclient.WithVerbose(ctx, &errBuf)

	cmd := listDefaultPipelinesCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("list-default-pipelines verbose failed: %v", err)
	}

	verbose := errBuf.String()
	if !strings.Contains(verbose, "GET") {
		t.Errorf("expected GET in verbose output; got: %q", verbose)
	}
	if !strings.Contains(verbose, "/api/account_pipeline_defaults") {
		t.Errorf("expected /api/account_pipeline_defaults in verbose output; got: %q", verbose)
	}
	if !strings.Contains(verbose, "200") {
		t.Errorf("expected status 200 in verbose output; got: %q", verbose)
	}
}

func TestListDefaultPipelinesPaginates(t *testing.T) {
	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.URL.RawQuery == "" {
			_, _ = fmt.Fprintf(w,
				`{"data":[{"type":"account_pipeline_defaults","id":"apd1","attributes":{"kind":"feature","pipeline_id":"1"}}],"links":{"next":"%s/api/account_pipeline_defaults?page%%5Bnumber%%5D=2"}}`,
				"http://"+r.Host)
		} else {
			_, _ = fmt.Fprint(w,
				`{"data":[{"type":"account_pipeline_defaults","id":"apd2","attributes":{"kind":"bugfix","pipeline_id":"2"}}],"links":{}}`)
		}
	}))
	defer ts.Close()

	var out strings.Builder
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := listDefaultPipelinesCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("list-default-pipelines pagination failed: %v", err)
	}
	if got := atomic.LoadInt32(&requestCount); got != 2 {
		t.Errorf("expected 2 requests (2 pages); got %d", got)
	}
	result := out.String()
	if !strings.Contains(result, "feature") {
		t.Errorf("output missing feature from page 1:\n%s", result)
	}
	if !strings.Contains(result, "bugfix") {
		t.Errorf("output missing bugfix from page 2:\n%s", result)
	}
}
