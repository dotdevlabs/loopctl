package taskkinds

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestConformance_TaskKindsSetDefaultPipeline(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	var gotContentType string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"task_kinds","id":"kind1","attributes":{"name":"feature","built_in":false,"default_pipeline_id":"pipe-abc"}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := setDefaultPipelineCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("pipeline-id", "pipe-abc")
	_ = cmd.RunE(cmd, []string{"kind1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for task-kinds set-default-pipeline: %v", violations)
	}
	if gotContentType != "application/json" {
		t.Errorf("set-default-pipeline Content-Type = %q; want application/json", gotContentType)
	}
}

func TestConformance_TaskKindsClearDefaultPipeline(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	var gotContentType string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"task_kinds","id":"kind1","attributes":{"name":"feature","built_in":false,"default_pipeline_id":""}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := clearDefaultPipelineCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, []string{"kind1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for task-kinds clear-default-pipeline: %v", violations)
	}
	if gotContentType != "application/json" {
		t.Errorf("clear-default-pipeline Content-Type = %q; want application/json", gotContentType)
	}
}

func TestConformance_ViolationDetection_ForbiddenTaskKindPatchField(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	// "built_in" is not a writable field in PATCH /api/task_kinds/:id.
	body := `{"task_kind":{"built_in":true}}`
	req, _ := http.NewRequest(http.MethodPatch, "http://x/api/task_kinds/kind1", strings.NewReader(body))
	violations := schema.CheckRequest(req, endpoints)
	if len(violations) == 0 {
		t.Fatal("expected violation for 'built_in' field in task_kinds PATCH; got none")
	}
}
