package pipelines

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
)

// pipelineWithStagesResponse returns a JSON:API pipeline response including stages.
func pipelineWithStagesResponse(id, name string, stages []map[string]any) string {
	stagesJSON, _ := json.Marshal(stages)
	return fmt.Sprintf(
		`{"data":{"type":"pipelines","id":%q,"attributes":{"name":%q,"kind":"my-kind","stages":%s}}}`,
		id, name, stagesJSON,
	)
}

// pipelineNoStagesResponse returns a JSON:API pipeline response with no stages.
func pipelineNoStagesResponse(id, name string) string {
	return fmt.Sprintf(
		`{"data":{"type":"pipelines","id":%q,"attributes":{"name":%q,"kind":"my-kind"}}}`,
		id, name,
	)
}

// ---- List tests ----

func TestStagesList_Happy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/pipelines/pipe1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, pipelineWithStagesResponse("pipe1", "my-pipeline", []map[string]any{
			{"name": "plan", "role": "planning", "stage_type": "ai", "gate": "manual", "agent": "my-agent"},
		}))
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := stagesListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"pipe1"}); err != nil {
		t.Fatalf("stages list failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "plan") {
		t.Errorf("output missing stage name 'plan':\n%s", got)
	}
	if !strings.Contains(got, "planning") {
		t.Errorf("output missing role 'planning':\n%s", got)
	}
	if !strings.Contains(got, "manual") {
		t.Errorf("output missing gate 'manual':\n%s", got)
	}
	if !strings.Contains(got, "my-agent") {
		t.Errorf("output missing agent 'my-agent':\n%s", got)
	}
}

func TestStagesList_EmptyStages(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, pipelineNoStagesResponse("pipe1", "my-pipeline"))
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := stagesListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"pipe1"}); err != nil {
		t.Fatalf("stages list empty failed: %v", err)
	}
	// Should render table headers without error.
	got := out.String()
	if !strings.Contains(got, "POSITION") {
		t.Errorf("expected POSITION column header; got:\n%s", got)
	}
}

func TestStagesList_JSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, pipelineWithStagesResponse("pipe1", "my-pipeline", []map[string]any{
			{"name": "plan", "role": "planning"},
		}))
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", true, &out)
	cmd := stagesListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"pipe1"}); err != nil {
		t.Fatalf("stages list json failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"id"`) {
		t.Errorf("expected JSON output with 'id' field; got:\n%s", got)
	}
	if !strings.Contains(got, "pipe1") {
		t.Errorf("expected pipeline id 'pipe1' in JSON output; got:\n%s", got)
	}
}

func TestStagesList_Error404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"404","detail":"pipeline not found"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := stagesListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{"missing"})
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if !strings.Contains(err.Error(), "pipeline not found") {
		t.Errorf("expected 'pipeline not found' in error; got: %v", err)
	}
}

// ---- Add tests ----

func TestStagesAdd_MinimalFlags(t *testing.T) {
	var reqCount int
	var gotPatchBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, pipelineWithStagesResponse("pipe1", "my-pipeline", []map[string]any{
				{"name": "existing", "role": "planning"},
			}))
		} else {
			gotPatchBody, _ = io.ReadAll(r.Body)
			_, _ = fmt.Fprint(w, pipelineNoStagesResponse("pipe1", "my-pipeline"))
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := stagesAddCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "new-stage"); err != nil {
		t.Fatal(err)
	}

	if err := cmd.RunE(cmd, []string{"pipe1"}); err != nil {
		t.Fatalf("stages add failed: %v", err)
	}
	if reqCount != 2 {
		t.Errorf("expected GET + PATCH (2 requests); got %d", reqCount)
	}

	// Verify new stage appended.
	var body map[string]json.RawMessage
	if err := json.Unmarshal(gotPatchBody, &body); err != nil {
		t.Fatalf("invalid PATCH body JSON: %v", err)
	}
	var pipeline map[string]json.RawMessage
	if err := json.Unmarshal(body["pipeline"], &pipeline); err != nil {
		t.Fatalf("invalid pipeline JSON: %v", err)
	}
	var stages []map[string]json.RawMessage
	if err := json.Unmarshal(pipeline["stages"], &stages); err != nil {
		t.Fatalf("invalid stages JSON: %v", err)
	}
	if len(stages) != 2 {
		t.Fatalf("expected 2 stages (existing + new); got %d", len(stages))
	}
}

func TestStagesAdd_AllFlags(t *testing.T) {
	var gotPatchBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, pipelineNoStagesResponse("pipe1", "my-pipeline"))
		} else {
			gotPatchBody, _ = io.ReadAll(r.Body)
			_, _ = fmt.Fprint(w, pipelineNoStagesResponse("pipe1", "my-pipeline"))
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := stagesAddCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	flags := map[string]string{
		"name":                 "full-stage",
		"role":                 "implementing",
		"stage-type":           "custom",
		"custom-stage-name":    "my-custom",
		"template":             "my-template",
		"instructions":         "do the thing",
		"gate":                 "manual",
		"agent":                "my-agent",
		"advance-notice":       "5m",
		"position":             "2",
		"on-failure":           `{"max_rework_count":3}`,
		"prompt-sections":      `[{"key":"k","value":"v"}]`,
		"stage-triggers":       `["trigger1"]`,
		"advance-requirements": `["req1"]`,
		"branch-conditions":    `["cond1"]`,
		"environment":          `{"KEY":"VALUE"}`,
	}
	for k, v := range flags {
		if err := cmd.Flags().Set(k, v); err != nil {
			t.Fatalf("setting flag --%s: %v", k, err)
		}
	}
	// runs-in-container is a bool flag
	if err := cmd.Flags().Set("runs-in-container", "true"); err != nil {
		t.Fatal(err)
	}

	if err := cmd.RunE(cmd, []string{"pipe1"}); err != nil {
		t.Fatalf("stages add all flags failed: %v", err)
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(gotPatchBody, &body); err != nil {
		t.Fatalf("invalid PATCH body JSON: %v\nbody: %s", err, gotPatchBody)
	}
	var pipeline map[string]json.RawMessage
	if err := json.Unmarshal(body["pipeline"], &pipeline); err != nil {
		t.Fatalf("invalid pipeline JSON: %v", err)
	}
	var stages []map[string]json.RawMessage
	if err := json.Unmarshal(pipeline["stages"], &stages); err != nil {
		t.Fatalf("invalid stages JSON: %v", err)
	}
	if len(stages) != 1 {
		t.Fatalf("expected 1 stage; got %d", len(stages))
	}
	stage := stages[0]

	for _, key := range []string{
		"name", "role", "stage_type", "custom_stage_name", "template",
		"instructions", "gate", "agent", "advance_notice", "position",
		"runs_in_container", "on_failure", "prompt_sections", "stage_triggers",
		"advance_requirements", "branch_conditions", "environment",
	} {
		if _, ok := stage[key]; !ok {
			t.Errorf("stage missing field %q; got keys: %v", key, mapKeys(stage))
		}
	}
}

func TestStagesAdd_DryRun(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{DryRun: true})
	cmd := stagesAddCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "new-stage"); err != nil {
		t.Fatal(err)
	}

	if err := cmd.RunE(cmd, []string{"pipe1"}); err != nil {
		t.Fatalf("stages add dry-run failed: %v", err)
	}
	if called {
		t.Error("dry-run should not make HTTP calls")
	}
	if !strings.Contains(out.String(), "dry-run") {
		t.Errorf("expected dry-run in output; got:\n%s", out.String())
	}
}

func TestStagesAdd_Error422(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, pipelineNoStagesResponse("pipe1", "my-pipeline"))
		} else {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = fmt.Fprint(w, `{"errors":[{"status":"422","detail":"stage name can't be blank"}]}`)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := stagesAddCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "bad-stage"); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, []string{"pipe1"})
	if err == nil {
		t.Fatal("expected error on 422")
	}
	if !strings.Contains(err.Error(), "stage name can't be blank") {
		t.Errorf("expected error detail; got: %v", err)
	}
}

func TestStagesAdd_BadJSONFlag(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := stagesAddCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "my-stage"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("on-failure", "not-valid-json"); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, []string{"pipe1"})
	if err == nil {
		t.Fatal("expected error for invalid --on-failure JSON")
	}
	if called {
		t.Error("HTTP call should not be made when --on-failure JSON is invalid")
	}
}

func TestStagesAdd_RequestShape(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, pipelineNoStagesResponse("p1", "x"))
		} else {
			gotBody, _ = io.ReadAll(r.Body)
			_, _ = fmt.Fprint(w, pipelineNoStagesResponse("p1", "x"))
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := stagesAddCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "my-stage"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, []string{"p1"}); err != nil {
		t.Fatalf("stages add failed: %v", err)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(gotBody, &top); err != nil {
		t.Fatalf("PATCH body not valid JSON: %v", err)
	}
	if _, ok := top["pipeline"]; !ok {
		t.Fatal("PATCH body missing 'pipeline' wrapper key")
	}
	var pipeline map[string]json.RawMessage
	if err := json.Unmarshal(top["pipeline"], &pipeline); err != nil {
		t.Fatalf("pipeline value not JSON object: %v", err)
	}
	if _, ok := pipeline["stages"]; !ok {
		t.Fatal("pipeline object missing 'stages' key")
	}
	var stages []map[string]json.RawMessage
	if err := json.Unmarshal(pipeline["stages"], &stages); err != nil {
		t.Fatalf("stages not JSON array: %v", err)
	}
	if len(stages) != 1 {
		t.Fatalf("expected 1 stage; got %d", len(stages))
	}
	if _, ok := stages[0]["name"]; !ok {
		t.Error("stage missing 'name' field")
	}
}

func TestStagesAdd_PreservesExistingStages(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, pipelineWithStagesResponse("p1", "x", []map[string]any{
				{"name": "stage-a", "role": "planning"},
				{"name": "stage-b", "role": "implementing"},
			}))
		} else {
			gotBody, _ = io.ReadAll(r.Body)
			_, _ = fmt.Fprint(w, pipelineNoStagesResponse("p1", "x"))
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := stagesAddCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "stage-c"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, []string{"p1"}); err != nil {
		t.Fatalf("stages add failed: %v", err)
	}

	var top map[string]json.RawMessage
	_ = json.Unmarshal(gotBody, &top)
	var pipeline map[string]json.RawMessage
	_ = json.Unmarshal(top["pipeline"], &pipeline)
	var stages []map[string]json.RawMessage
	_ = json.Unmarshal(pipeline["stages"], &stages)

	if len(stages) != 3 {
		t.Fatalf("expected 3 stages (2 existing + 1 new); got %d", len(stages))
	}
}

func TestStagesAdd_OnFailure(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, pipelineNoStagesResponse("p1", "x"))
		} else {
			gotBody, _ = io.ReadAll(r.Body)
			_, _ = fmt.Fprint(w, pipelineNoStagesResponse("p1", "x"))
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := stagesAddCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "my-stage"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("on-failure", `{"max_rework_count":3,"rework_to_position":0}`); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, []string{"p1"}); err != nil {
		t.Fatalf("stages add with on-failure failed: %v", err)
	}

	var top map[string]json.RawMessage
	_ = json.Unmarshal(gotBody, &top)
	var pipeline map[string]json.RawMessage
	_ = json.Unmarshal(top["pipeline"], &pipeline)
	var stages []map[string]json.RawMessage
	_ = json.Unmarshal(pipeline["stages"], &stages)
	if len(stages) != 1 {
		t.Fatalf("expected 1 stage; got %d", len(stages))
	}
	if _, ok := stages[0]["on_failure"]; !ok {
		t.Error("stage missing 'on_failure' field; got keys: " + strings.Join(mapStringKeys(stages[0]), ", "))
	}
}

// ---- Update tests ----

func TestStagesUpdate_ByName(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, pipelineWithStagesResponse("p1", "x", []map[string]any{
				{"name": "plan", "role": "planning", "instructions": "old instructions"},
			}))
		} else {
			gotBody, _ = io.ReadAll(r.Body)
			_, _ = fmt.Fprint(w, pipelineNoStagesResponse("p1", "x"))
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := stagesUpdateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("instructions", "new instructions"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, []string{"p1", "plan"}); err != nil {
		t.Fatalf("stages update failed: %v", err)
	}

	var top map[string]json.RawMessage
	_ = json.Unmarshal(gotBody, &top)
	var pipeline map[string]json.RawMessage
	_ = json.Unmarshal(top["pipeline"], &pipeline)
	var stages []map[string]json.RawMessage
	_ = json.Unmarshal(pipeline["stages"], &stages)

	if len(stages) != 1 {
		t.Fatalf("expected 1 stage; got %d", len(stages))
	}
	var instructions string
	_ = json.Unmarshal(stages[0]["instructions"], &instructions)
	if instructions != "new instructions" {
		t.Errorf("expected instructions='new instructions'; got %q", instructions)
	}
}

func TestStagesUpdate_OnlyChangedFlags(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, pipelineWithStagesResponse("p1", "x", []map[string]any{
				{"name": "plan", "role": "planning", "gate": "auto"},
			}))
		} else {
			gotBody, _ = io.ReadAll(r.Body)
			_, _ = fmt.Fprint(w, pipelineNoStagesResponse("p1", "x"))
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := stagesUpdateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("gate", "manual"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, []string{"p1", "plan"}); err != nil {
		t.Fatalf("stages update failed: %v", err)
	}

	var top map[string]json.RawMessage
	_ = json.Unmarshal(gotBody, &top)
	var pipeline map[string]json.RawMessage
	_ = json.Unmarshal(top["pipeline"], &pipeline)
	var stages []map[string]json.RawMessage
	_ = json.Unmarshal(pipeline["stages"], &stages)

	// "role" was in the original stage, should still be there.
	if _, ok := stages[0]["role"]; !ok {
		t.Error("stage should preserve 'role' from original; got keys: " + strings.Join(mapStringKeys(stages[0]), ", "))
	}
	var gate string
	_ = json.Unmarshal(stages[0]["gate"], &gate)
	if gate != "manual" {
		t.Errorf("expected gate='manual'; got %q", gate)
	}
}

func TestStagesUpdate_PreservesUnknownFields(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.Method == http.MethodGet {
			// Stage has server-generated fields not in StageInput.
			_, _ = fmt.Fprint(w, pipelineWithStagesResponse("p1", "x", []map[string]any{
				{"name": "plan", "role": "planning", "server_id": "srv-123", "created_at": "2024-01-01"},
			}))
		} else {
			gotBody, _ = io.ReadAll(r.Body)
			_, _ = fmt.Fprint(w, pipelineNoStagesResponse("p1", "x"))
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := stagesUpdateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("gate", "manual"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, []string{"p1", "plan"}); err != nil {
		t.Fatalf("stages update failed: %v", err)
	}

	var top map[string]json.RawMessage
	_ = json.Unmarshal(gotBody, &top)
	var pipeline map[string]json.RawMessage
	_ = json.Unmarshal(top["pipeline"], &pipeline)
	var stages []map[string]json.RawMessage
	_ = json.Unmarshal(pipeline["stages"], &stages)

	if _, ok := stages[0]["server_id"]; !ok {
		t.Error("stage should preserve unknown 'server_id' field from server")
	}
	if _, ok := stages[0]["created_at"]; !ok {
		t.Error("stage should preserve unknown 'created_at' field from server")
	}
}

func TestStagesUpdate_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, pipelineWithStagesResponse("p1", "x", []map[string]any{
			{"name": "plan", "role": "planning"},
		}))
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := stagesUpdateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("role", "reviewing"); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, []string{"p1", "nonexistent-stage"})
	if err == nil {
		t.Fatal("expected error for nonexistent stage")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error; got: %v", err)
	}
}

func TestStagesUpdate_DryRun(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{DryRun: true})
	cmd := stagesUpdateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("role", "reviewing"); err != nil {
		t.Fatal(err)
	}

	if err := cmd.RunE(cmd, []string{"p1", "plan"}); err != nil {
		t.Fatalf("stages update dry-run failed: %v", err)
	}
	if called {
		t.Error("dry-run should not make HTTP calls")
	}
	if !strings.Contains(out.String(), "dry-run") {
		t.Errorf("expected dry-run in output; got:\n%s", out.String())
	}
}

func TestStagesUpdate_NoFlags(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := stagesUpdateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{"p1", "plan"})
	if err == nil {
		t.Fatal("expected error when no flags provided")
	}
	if called {
		t.Error("HTTP call should not be made when no flags provided")
	}
}

// ---- Remove tests ----

func TestStagesRemove_ByName(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, pipelineWithStagesResponse("p1", "x", []map[string]any{
				{"name": "plan", "role": "planning"},
				{"name": "implement", "role": "implementing"},
			}))
		} else {
			gotBody, _ = io.ReadAll(r.Body)
			_, _ = fmt.Fprint(w, pipelineNoStagesResponse("p1", "x"))
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := stagesRemoveCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"p1", "plan"}); err != nil {
		t.Fatalf("stages remove failed: %v", err)
	}

	var top map[string]json.RawMessage
	_ = json.Unmarshal(gotBody, &top)
	var pipeline map[string]json.RawMessage
	_ = json.Unmarshal(top["pipeline"], &pipeline)
	var stages []map[string]json.RawMessage
	_ = json.Unmarshal(pipeline["stages"], &stages)

	if len(stages) != 1 {
		t.Fatalf("expected 1 stage remaining after remove; got %d", len(stages))
	}
	var name string
	_ = json.Unmarshal(stages[0]["name"], &name)
	if name != "implement" {
		t.Errorf("expected remaining stage to be 'implement'; got %q", name)
	}
}

func TestStagesRemove_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, pipelineWithStagesResponse("p1", "x", []map[string]any{
			{"name": "plan", "role": "planning"},
		}))
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := stagesRemoveCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{"p1", "nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent stage")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error; got: %v", err)
	}
}

func TestStagesRemove_LastStage(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, pipelineWithStagesResponse("p1", "x", []map[string]any{
				{"name": "only-stage", "role": "planning"},
			}))
		} else {
			gotBody, _ = io.ReadAll(r.Body)
			_, _ = fmt.Fprint(w, pipelineNoStagesResponse("p1", "x"))
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := stagesRemoveCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"p1", "only-stage"}); err != nil {
		t.Fatalf("stages remove last stage failed: %v", err)
	}

	// Verify empty array [] is sent, not null.
	var top map[string]json.RawMessage
	_ = json.Unmarshal(gotBody, &top)
	var pipeline map[string]json.RawMessage
	_ = json.Unmarshal(top["pipeline"], &pipeline)

	stagesRaw := string(pipeline["stages"])
	if stagesRaw != "[]" {
		t.Errorf("expected empty array '[]'; got %q", stagesRaw)
	}
}

func TestStagesRemove_DryRun(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{DryRun: true})
	cmd := stagesRemoveCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"p1", "plan"}); err != nil {
		t.Fatalf("stages remove dry-run failed: %v", err)
	}
	if called {
		t.Error("dry-run should not make HTTP calls")
	}
	if !strings.Contains(out.String(), "dry-run") {
		t.Errorf("expected dry-run in output; got:\n%s", out.String())
	}
}

// ---- JSON:API error tests ----

func TestStagesAdd_GetPipelineError(t *testing.T) {
	var patchCalled bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, `{"errors":[{"status":"500","detail":"server error on GET"}]}`)
		} else {
			patchCalled = true
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := stagesAddCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("name", "my-stage"); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, []string{"p1"})
	if err == nil {
		t.Fatal("expected error on GET failure")
	}
	if patchCalled {
		t.Error("PATCH should not be called if GET fails")
	}
	if !strings.Contains(err.Error(), "server error on GET") {
		t.Errorf("expected GET error detail in error; got: %v", err)
	}
}

func TestStagesUpdate_GetPipelineError(t *testing.T) {
	var patchCalled bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"errors":[{"status":"404","detail":"pipeline not found on GET"}]}`)
		} else {
			patchCalled = true
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := stagesUpdateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("role", "reviewing"); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, []string{"p1", "plan"})
	if err == nil {
		t.Fatal("expected error on GET failure")
	}
	if patchCalled {
		t.Error("PATCH should not be called if GET fails")
	}
}

// mapStringKeys returns the keys of a map[string]json.RawMessage as a sorted slice.
func mapStringKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
