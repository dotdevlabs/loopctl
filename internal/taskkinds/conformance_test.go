package taskkinds

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dotdevlabs/loopctl/internal/schema"
)

func loadSchemaOrSkip(t *testing.T) []schema.EndpointAttrs {
	t.Helper()
	endpoints, err := schema.Load()
	if err != nil {
		t.Skipf("cannot load schema fixture: %v", err)
	}
	return endpoints
}

func TestConformance_TaskKindsList(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[]}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, nil)

	if len(violations) != 0 {
		t.Errorf("conformance violations for task-kinds list: %v", violations)
	}
}

func TestConformance_TaskKindsCreate(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	var gotContentType string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":{"type":"task_kinds","id":"k1","attributes":{"name":"my-kind","built_in":false}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("name", "my-kind")
	_ = cmd.RunE(cmd, nil)

	if len(violations) != 0 {
		t.Errorf("conformance violations for task-kinds create: %v", violations)
	}
	if gotContentType != "application/json" {
		t.Errorf("task-kinds create Content-Type = %q; want application/json", gotContentType)
	}
}

func TestConformance_ViolationDetection_ForbiddenTaskKindField(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	// "built_in" is not a writable field in POST /api/task_kinds.
	body := `{"task_kind":{"name":"my-kind","built_in":true}}`
	req, _ := http.NewRequest(http.MethodPost, "http://x/api/task_kinds", strings.NewReader(body))
	violations := schema.CheckRequest(req, endpoints)
	if len(violations) == 0 {
		t.Fatal("expected violation for 'built_in' field in task_kinds create; got none")
	}
}

// TestConformance_TaskKindsSetDefaultPipeline verifies PATCH /api/account_pipeline_defaults/{kind}.
func TestConformance_TaskKindsSetDefaultPipeline(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	var gotContentType string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"account_pipeline_defaults","id":"apd1","attributes":{"kind":"feature","pipeline_id":"123"}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := setDefaultPipelineCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("pipeline-id", "123")
	_ = cmd.RunE(cmd, []string{"feature"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for task-kinds set-default-pipeline: %v", violations)
	}
	if gotContentType != "application/json" {
		t.Errorf("set-default-pipeline Content-Type = %q; want application/json", gotContentType)
	}
}

// TestConformance_TaskKindsClearDefaultPipeline verifies DELETE /api/account_pipeline_defaults/{kind}.
func TestConformance_TaskKindsClearDefaultPipeline(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	var gotMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		violations = schema.CheckRequest(r, endpoints)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := clearDefaultPipelineCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, []string{"feature"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for task-kinds clear-default-pipeline: %v", violations)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("clear-default-pipeline method = %q; want DELETE", gotMethod)
	}
}

// TestConformance_ViolationDetection_ForbiddenTaskKindPatchField verifies that
// PATCH /api/task_kinds/:id is NOT in the spec and any request to it is a violation.
func TestConformance_ViolationDetection_ForbiddenTaskKindPatchField(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	// PATCH /api/task_kinds/:id is not in the spec at all.
	body := `{"task_kind":{"built_in":true}}`
	req, _ := http.NewRequest(http.MethodPatch, "http://x/api/task_kinds/kind1", strings.NewReader(body))
	violations := schema.CheckRequest(req, endpoints)
	if len(violations) == 0 {
		t.Fatal("expected violation for PATCH /api/task_kinds/:id (endpoint not in spec); got none")
	}
}

// TestConformance_ListDefaultPipelines verifies GET /api/account_pipeline_defaults conforms to the spec.
func TestConformance_ListDefaultPipelines(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[],"links":{},"meta":{"total":0,"page":1,"per_page":20}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := listDefaultPipelinesCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, nil)

	if len(violations) != 0 {
		t.Errorf("conformance violations for list-default-pipelines: %v", violations)
	}
}

func TestConformance_TaskKindsListPaginates(t *testing.T) {
	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.URL.RawQuery == "" {
			_, _ = fmt.Fprintf(w,
				`{"data":[{"type":"task_kinds","id":"k1","attributes":{"name":"feature","built_in":true}}],"links":{"next":"%s/api/task_kinds?page%%5Bnumber%%5D=2"}}`,
				"http://"+r.Host)
		} else {
			_, _ = fmt.Fprint(w,
				`{"data":[{"type":"task_kinds","id":"k2","attributes":{"name":"bugfix","built_in":false}}],"links":{}}`)
		}
	}))
	defer ts.Close()

	var out strings.Builder
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("task-kinds list with pagination failed: %v", err)
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
