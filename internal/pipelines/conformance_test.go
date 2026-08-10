package pipelines

import (
	"encoding/json"
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

func TestConformance_PipelinesList(t *testing.T) {
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
		t.Errorf("conformance violations for pipelines list: %v", violations)
	}
}

func TestConformance_PipelinesCreate(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	var gotContentType string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"p1","attributes":{"name":"my-pipeline","kind":""}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("name", "my-pipeline")
	_ = cmd.RunE(cmd, nil)

	if len(violations) != 0 {
		t.Errorf("conformance violations for pipelines create: %v", violations)
	}
	if gotContentType != "application/json" {
		t.Errorf("pipelines create Content-Type = %q; want application/json", gotContentType)
	}
}

func TestConformance_ViolationDetection_ForbiddenPipelineField(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	// "display_name" is not an allowed field for POST /api/pipelines.
	body := `{"pipeline":{"name":"my-pipeline","display_name":"bad"}}`
	req, _ := http.NewRequest(http.MethodPost, "http://x/api/pipelines", strings.NewReader(body))
	violations := schema.CheckRequest(req, endpoints)
	if len(violations) == 0 {
		t.Fatal("expected violation for forbidden 'display_name' field in pipelines create; got none")
	}
	found := false
	for _, v := range violations {
		if strings.Contains(v, "display_name") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'display_name' in violations; got: %v", violations)
	}
}

func TestConformance_PipelinesCreate_WithStages(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"p1","attributes":{"name":"my-pipeline","kind":""}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("name", "my-pipeline")
	_ = cmd.Flags().Set("stages", `[{"name":"plan","role":"planning","instructions":"Plan the work."}]`)
	_ = cmd.RunE(cmd, nil)

	if len(violations) != 0 {
		t.Errorf("conformance violations for pipelines create with stages: %v", violations)
	}
}

func TestConformance_PipelinesUpdate(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	var gotContentType string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"p1","attributes":{"name":"updated","kind":""}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("name", "updated")
	_ = cmd.RunE(cmd, []string{"p1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for pipelines update: %v", violations)
	}
	if gotContentType != "application/json" {
		t.Errorf("pipelines update Content-Type = %q; want application/json", gotContentType)
	}
}

func TestConformance_PipelinesUpdate_WithStages(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"p1","attributes":{"name":"my-pipeline","kind":""}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("stages", `[{"name":"plan","role":"planning","instructions":"Plan."},{"name":"implement","role":"implementing","instructions":"Implement."}]`)
	_ = cmd.RunE(cmd, []string{"p1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for pipelines update with stages: %v", violations)
	}
}

// TestConformance_StageFieldsPermissive verifies that stage items accept any keys.
// The spec defines stages as array of type: object with no properties, so any
// key in a stage item is valid and must not trigger a conformance violation.
func TestConformance_StageFieldsPermissive(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	body := `{"pipeline":{"name":"x","stages":[{"name":"plan","role":"planning","instructions":"i","arbitrary_key":"x"}]}}`
	req, _ := http.NewRequest(http.MethodPost, "http://x/api/pipelines", strings.NewReader(body))
	violations := schema.CheckRequest(req, endpoints)
	if len(violations) != 0 {
		t.Errorf("spec defines stages items as open objects — arbitrary stage keys must not produce violations; got: %v", violations)
	}
}

func TestConformance_PipelinesList_ReturnsJSON(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[]}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", true, io.Discard)
	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, nil)

	if len(violations) != 0 {
		t.Errorf("conformance violations for pipelines list (json mode): %v", violations)
	}
}

func TestConformance_PipelinesCreate_AllFields(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"p1","attributes":{"name":"full-pipeline","kind":"my-kind"}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("name", "full-pipeline")
	_ = cmd.Flags().Set("kind", "my-kind")
	_ = cmd.Flags().Set("description", "A description")
	_ = cmd.Flags().Set("stages", `[{"name":"plan","role":"planning","instructions":"Plan."}]`)
	_ = cmd.RunE(cmd, nil)

	if len(violations) != 0 {
		t.Errorf("conformance violations for pipelines create with all fields: %v", violations)
	}
}

func TestConformance_CheckRequestStageParsing(t *testing.T) {
	// Directly verify CheckRequest parses stages for schema validation
	endpoints := loadSchemaOrSkip(t)
	body := `{"pipeline":{"name":"x","stages":[{"name":"plan","role":"planning","instructions":"i"}]}}`
	req, _ := http.NewRequest(http.MethodPost, "http://x/api/pipelines", strings.NewReader(body))
	violations := schema.CheckRequest(req, endpoints)
	if len(violations) != 0 {
		t.Errorf("expected no violations for valid stages; got: %v", violations)
	}

	// Also verify the body was not consumed
	var buf strings.Builder
	_, _ = io.Copy(&buf, req.Body)
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(buf.String()), &parsed); err != nil {
		t.Errorf("body was not restored after CheckRequest: %v", err)
	}
}
