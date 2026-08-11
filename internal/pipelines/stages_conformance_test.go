package pipelines

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
	_ = cmd.Flags().Set("template", "my-template")
	_ = cmd.Flags().Set("instructions", "do the thing")
	_ = cmd.Flags().Set("gate", "manual")
	_ = cmd.Flags().Set("agent", "my-agent")
	_ = cmd.Flags().Set("advance-notice", "5m")
	_ = cmd.Flags().Set("position", "2")
	_ = cmd.Flags().Set("runs-in-container", "true")
	_ = cmd.Flags().Set("on-failure", `{"max_rework_count":3}`)
	_ = cmd.Flags().Set("prompt-sections", `[{"key":"k","value":"v"}]`)
	_ = cmd.Flags().Set("stage-triggers", `["trigger1"]`)
	_ = cmd.Flags().Set("advance-requirements", `["req1"]`)
	_ = cmd.Flags().Set("branch-conditions", `["cond1"]`)
	_ = cmd.Flags().Set("environment", `{"KEY":"VALUE"}`)

	_ = cmd.RunE(cmd, []string{"p1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for stages add with all attrs (spec defines stages as open objects): %v", violations)
	}
}

func TestConformance_StagesUpdate(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, pipelineWithStagesResponse("p1", "x", []map[string]any{
				{"name": "plan", "role": "planning"},
			}))
		} else {
			violations = schema.CheckRequest(r, endpoints)
			_, _ = fmt.Fprint(w, pipelineNoStagesResponse("p1", "x"))
		}
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := stagesUpdateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("role", "reviewing")
	_ = cmd.RunE(cmd, []string{"p1", "plan"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for stages update: %v", violations)
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
