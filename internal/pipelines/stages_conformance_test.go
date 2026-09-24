package pipelines

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dotdevlabs/loopctl/internal/schema"
)

func TestConformance_StagesList(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, pipelineWithStagesResponse("p1", "x", nil))
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := stagesListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, []string{"p1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for stages list: %v", violations)
	}
}

func TestConformance_StagesAdd(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, pipelineNoStagesResponse("p1", "x"))
		} else {
			violations = schema.CheckRequest(r, endpoints)
			_, _ = fmt.Fprint(w, pipelineNoStagesResponse("p1", "x"))
		}
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := stagesAddCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("name", "my-stage")
	_ = cmd.RunE(cmd, []string{"p1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for stages add: %v", violations)
	}
}

func TestConformance_StagesAdd_AllAttrs(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, pipelineNoStagesResponse("p1", "x"))
		} else {
			violations = schema.CheckRequest(r, endpoints)
			_, _ = fmt.Fprint(w, pipelineNoStagesResponse("p1", "x"))
		}
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := stagesAddCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)

	_ = cmd.Flags().Set("name", "full-stage")
	_ = cmd.Flags().Set("role", "implementing")
	_ = cmd.Flags().Set("stage-type", "custom")
	_ = cmd.Flags().Set("custom-stage-name", "my-custom")
	_ = cmd.Flags().Set("instructions", "do the thing")
	_ = cmd.Flags().Set("gate", "automated")
	_ = cmd.Flags().Set("advance-notice", "5m")
	_ = cmd.Flags().Set("position", "2")
	_ = cmd.Flags().Set("runs-in-container", "true")
	_ = cmd.Flags().Set("on-failure", "rework")
	_ = cmd.Flags().Set("max-rework-count", "3")
	_ = cmd.Flags().Set("prompt-sections", `{"template":"my-template"}`)
	_ = cmd.Flags().Set("stage-triggers", `[{"timing":"before_entry","handler":"MyHandler","config":{}}]`)
	_ = cmd.Flags().Set("advance-requirements", `["req1"]`)

	_ = cmd.RunE(cmd, []string{"p1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for stages add with all spec-compliant attrs: %v", violations)
	}
}

func TestConformance_StagesUpdate(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH; got %s", r.Method)
		}
		if r.URL.Path != "/api/pipelines/p1/stages/s1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, stageResourceResponse("s1", map[string]any{
			"role": "reviewing", "stage_type": "ai", "gate": "automated", "position": 1,
		}))
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := stagesUpdateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("role", "reviewing")
	_ = cmd.RunE(cmd, []string{"p1", "s1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for stages update: %v", violations)
	}
}

func TestConformance_StagesUpdate_NullName(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		// Built-in stage returns name: null.
		_, _ = fmt.Fprint(w, stageResourceResponse("s1", map[string]any{
			"name": nil, "role": "reviewing", "stage_type": "ai", "gate": "automated", "position": 1,
		}))
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := stagesUpdateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("role", "reviewing")
	err := cmd.RunE(cmd, []string{"p1", "s1"})
	if err != nil {
		t.Fatalf("update with null name returned error: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("conformance violations for stages update null name: %v", violations)
	}
}

func TestConformance_StagesUpdate_Error404(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"404","detail":"stage not found by id"}]}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := stagesUpdateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("role", "reviewing")
	err := cmd.RunE(cmd, []string{"p1", "s1"})
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if !strings.Contains(err.Error(), "stage not found by id") {
		t.Errorf("expected detail in error; got: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("conformance violations for stages update 404: %v", violations)
	}
}

func TestConformance_StagesUpdate_Error422(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"422","detail":"gate must be automated or manual"}]}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := stagesUpdateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("role", "reviewing")
	err := cmd.RunE(cmd, []string{"p1", "s1"})
	if err == nil {
		t.Fatal("expected error on 422")
	}
	if !strings.Contains(err.Error(), "gate must be automated or manual") {
		t.Errorf("expected detail in error; got: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("conformance violations for stages update 422: %v", violations)
	}
}

func TestConformance_StagesRemove(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, pipelineWithStagesResponse("p1", "x", []map[string]any{
				{"name": "plan", "role": "planning"},
				{"name": "implement", "role": "implementing"},
			}))
		} else {
			violations = schema.CheckRequest(r, endpoints)
			_, _ = fmt.Fprint(w, pipelineNoStagesResponse("p1", "x"))
		}
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := stagesRemoveCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, []string{"p1", "plan"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for stages remove: %v", violations)
	}
}
