package schema

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"encoding/json"
	"gopkg.in/yaml.v3"
)

func TestLoad(t *testing.T) {
	endpoints, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(endpoints) == 0 {
		t.Fatal("Load() returned no endpoints")
	}
	// Verify a few known endpoints are present.
	found := map[string]bool{}
	for _, ep := range endpoints {
		key := ep.Method + " " + ep.Path
		found[key] = true
	}
	for _, want := range []string{
		"POST /api/tasks",
		"GET /api/tasks/:id",
		"PATCH /api/tasks/:id",
		"POST /api/tasks/:task_id/cancellation",
		"GET /api/tasks/:task_id/activities",
		"POST /api/projects",
		"POST /api/pipelines",
		"PATCH /api/pipelines/:id",
		"POST /api/task_kinds",
		"PATCH /api/account_pipeline_defaults/:kind",
		"DELETE /api/account_pipeline_defaults/:kind",
	} {
		if !found[want] {
			t.Errorf("Load() missing endpoint %q", want)
		}
	}
	// PATCH /api/task_kinds/:id must NOT be present (not in spec).
	if found["PATCH /api/task_kinds/:id"] {
		t.Error("Load() must not contain PATCH /api/task_kinds/:id (not in spec)")
	}
}

func TestMatchPath(t *testing.T) {
	cases := []struct {
		template string
		actual   string
		want     bool
	}{
		{"/api/tasks", "/api/tasks", true},
		{"/api/tasks/:id", "/api/tasks/abc123", true},
		{"/api/tasks/:task_id/cancellation", "/api/tasks/t1/cancellation", true},
		{"/api/tasks/:task_id/activities", "/api/tasks/t1/activities", true},
		{"/api/tasks", "/api/projects", false},
		{"/api/tasks/:id", "/api/tasks/a/b", false},
		{"/api/tasks/:id", "/api/tasks", false},
	}
	for _, tc := range cases {
		got := MatchPath(tc.template, tc.actual)
		if got != tc.want {
			t.Errorf("MatchPath(%q, %q) = %v; want %v", tc.template, tc.actual, got, tc.want)
		}
	}
}

func TestCheckRequest_NoViolation(t *testing.T) {
	endpoints, _ := Load()
	body := `{"task":{"project_id":"p1","kind":"feature","title":"My task","description":"Details"}}`
	req, _ := http.NewRequest(http.MethodPost, "http://x/api/tasks", strings.NewReader(body))
	violations := CheckRequest(req, endpoints)
	if len(violations) != 0 {
		t.Errorf("expected no violations; got: %v", violations)
	}
}

func TestCheckRequest_UnknownEndpoint(t *testing.T) {
	endpoints, _ := Load()
	req, _ := http.NewRequest(http.MethodGet, "http://x/api/nonexistent", nil)
	violations := CheckRequest(req, endpoints)
	if len(violations) == 0 {
		t.Fatal("expected violation for unknown endpoint; got none")
	}
	if !strings.Contains(violations[0], "not in schema") {
		t.Errorf("expected 'not in schema' violation; got: %v", violations)
	}
}

func TestCheckRequest_ForbiddenField(t *testing.T) {
	endpoints, _ := Load()
	// tasks create does not allow "forbidden_field"
	body := `{"task":{"project_id":"p1","kind":"feature","title":"T","description":"D","forbidden_field":"bad"}}`
	req, _ := http.NewRequest(http.MethodPost, "http://x/api/tasks", strings.NewReader(body))
	violations := CheckRequest(req, endpoints)
	if len(violations) == 0 {
		t.Fatal("expected violation for forbidden field; got none")
	}
	found := false
	for _, v := range violations {
		if strings.Contains(v, "forbidden_field") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'forbidden_field' in violations; got: %v", violations)
	}
}

func TestCheckRequest_WrongNesting(t *testing.T) {
	endpoints, _ := Load()
	// Sending fields at top level instead of under "task" nesting.
	body := `{"project_id":"p1","kind":"feature","title":"T"}`
	req, _ := http.NewRequest(http.MethodPost, "http://x/api/tasks", strings.NewReader(body))
	violations := CheckRequest(req, endpoints)
	if len(violations) == 0 {
		t.Fatal("expected violation for wrong nesting; got none")
	}
}

func TestCheckRequest_NullBodyEndpoint_NoBody(t *testing.T) {
	endpoints, _ := Load()
	// POST /api/tasks/:task_id/cancellation has no requestBody — no body is correct.
	req, _ := http.NewRequest(http.MethodPost, "http://x/api/tasks/t1/cancellation", nil)
	violations := CheckRequest(req, endpoints)
	if len(violations) != 0 {
		t.Errorf("expected no violations for null-body endpoint with no body; got: %v", violations)
	}
}

func TestCheckRequest_NullBodyEndpoint_WithBody(t *testing.T) {
	endpoints, _ := Load()
	// Sending a body to an endpoint that declares request_body null is a violation.
	body := `{"unexpected":"field"}`
	req, _ := http.NewRequest(http.MethodPost, "http://x/api/tasks/t1/cancellation", strings.NewReader(body))
	violations := CheckRequest(req, endpoints)
	if len(violations) == 0 {
		t.Fatal("expected violation for body sent to null-body endpoint; got none")
	}
}

func TestCheckRequest_BodyRestoredAfterRead(t *testing.T) {
	endpoints, _ := Load()
	body := `{"task":{"project_id":"p1","kind":"feature","title":"T","description":"D"}}`
	req, _ := http.NewRequest(http.MethodPost, "http://x/api/tasks", strings.NewReader(body))
	_ = CheckRequest(req, endpoints)

	// Body should be restored and readable again.
	var buf bytes.Buffer
	_, err := buf.ReadFrom(req.Body)
	if err != nil {
		t.Fatalf("failed to read body after CheckRequest: %v", err)
	}
	if buf.String() != body {
		t.Errorf("body not restored: got %q; want %q", buf.String(), body)
	}
}

func TestCheckRequest_UpdateTask_ContractFields(t *testing.T) {
	endpoints, _ := Load()
	// PATCH /api/tasks/:id only allows implementation_criteria, verification_criteria, input_tokens, output_tokens.
	body := `{"task":{"implementation_criteria":"do it","verification_criteria":"check it"}}`
	req, _ := http.NewRequest(http.MethodPatch, "http://x/api/tasks/t1", strings.NewReader(body))
	violations := CheckRequest(req, endpoints)
	if len(violations) != 0 {
		t.Errorf("expected no violations for valid update fields; got: %v", violations)
	}
}

func TestCheckRequest_UpdateTask_ForbiddenOldFields(t *testing.T) {
	endpoints, _ := Load()
	// Old fields: kind, title, description — all forbidden in PATCH /api/tasks/:id.
	body := `{"task":{"kind":"feature","title":"My Task","description":"Details"}}`
	req, _ := http.NewRequest(http.MethodPatch, "http://x/api/tasks/t1", strings.NewReader(body))
	violations := CheckRequest(req, endpoints)
	if len(violations) == 0 {
		t.Fatal("expected violations for forbidden task update fields (kind, title, description); got none")
	}
}

// TestCheckRequest_StageFields verifies that stage items in pipeline create/update
// are allowed without field-level restrictions (spec defines stages as array of
// type: object with no properties, so any keys are valid).
func TestCheckRequest_StageFields_NoViolation(t *testing.T) {
	endpoints, _ := Load()
	body := `{"pipeline":{"name":"x","stages":[{"name":"plan","role":"planning","instructions":"i"}]}}`
	req, _ := http.NewRequest(http.MethodPost, "http://x/api/pipelines", strings.NewReader(body))
	violations := CheckRequest(req, endpoints)
	if len(violations) != 0 {
		t.Errorf("expected no violations for stages with any field names; got: %v", violations)
	}
}

// TestCheckRequest_StageFields_AnyKeyAllowed verifies that unknown stage item keys
// produce no violations (spec defines items as open objects with no properties).
func TestCheckRequest_StageFields_AnyKeyAllowed(t *testing.T) {
	endpoints, _ := Load()
	body := `{"pipeline":{"name":"x","stages":[{"name":"plan","role":"planning","instructions":"i","arbitrary_key":"x"}]}}`
	req, _ := http.NewRequest(http.MethodPost, "http://x/api/pipelines", strings.NewReader(body))
	violations := CheckRequest(req, endpoints)
	if len(violations) != 0 {
		t.Errorf("expected no violations for arbitrary stage keys (spec is permissive); got: %v", violations)
	}
}

func TestCheckRequest_AccountPipelineDefault_NoViolation(t *testing.T) {
	endpoints, _ := Load()
	body := `{"account_pipeline_default":{"pipeline_id":123}}`
	req, _ := http.NewRequest(http.MethodPatch, "http://x/api/account_pipeline_defaults/feature", strings.NewReader(body))
	violations := CheckRequest(req, endpoints)
	if len(violations) != 0 {
		t.Errorf("expected no violations for account_pipeline_default patch; got: %v", violations)
	}
}

func TestCheckRequest_AccountPipelineDefault_ForbiddenField(t *testing.T) {
	endpoints, _ := Load()
	body := `{"account_pipeline_default":{"pipeline_id":123,"bad_field":"x"}}`
	req, _ := http.NewRequest(http.MethodPatch, "http://x/api/account_pipeline_defaults/feature", strings.NewReader(body))
	violations := CheckRequest(req, endpoints)
	if len(violations) == 0 {
		t.Fatal("expected violation for bad_field in account_pipeline_default; got none")
	}
}

func TestCheckRequest_DeleteAccountPipelineDefault_NoBody(t *testing.T) {
	endpoints, _ := Load()
	req, _ := http.NewRequest(http.MethodDelete, "http://x/api/account_pipeline_defaults/feature", nil)
	violations := CheckRequest(req, endpoints)
	if len(violations) != 0 {
		t.Errorf("expected no violations for DELETE account_pipeline_defaults; got: %v", violations)
	}
}

func TestLoad_QueryParams(t *testing.T) {
	endpoints, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	var tasksGet *EndpointAttrs
	for i := range endpoints {
		ep := &endpoints[i]
		if ep.Method == "GET" && ep.Path == "/api/tasks" {
			tasksGet = ep
			break
		}
	}
	if tasksGet == nil {
		t.Fatal("GET /api/tasks not found in endpoints")
	}
	qpMap := make(map[string]QueryParam, len(tasksGet.QueryParams))
	for _, qp := range tasksGet.QueryParams {
		qpMap[qp.Name] = qp
	}
	for _, want := range []string{"project_id", "page[number]", "page[size]"} {
		if _, ok := qpMap[want]; !ok {
			t.Errorf("GET /api/tasks missing query param %q; got %v", want, tasksGet.QueryParams)
		}
	}
}

func TestLoad_IsPaginated(t *testing.T) {
	endpoints, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	type want struct {
		method    string
		path      string
		paginated bool
	}
	cases := []want{
		{"GET", "/api/tasks", true},
		{"GET", "/api/pipelines", true},
		{"GET", "/api/tasks/:task_id/todos", true},
		{"GET", "/api/tasks/:task_id/activities", true},
		{"POST", "/api/tasks", false},
		{"GET", "/api/tasks/:id", false},
		{"PATCH", "/api/tasks/:id", false},
	}
	epMap := make(map[string]EndpointAttrs, len(endpoints))
	for _, ep := range endpoints {
		epMap[ep.Method+" "+ep.Path] = ep
	}
	for _, tc := range cases {
		key := tc.method + " " + tc.path
		ep, ok := epMap[key]
		if !ok {
			t.Errorf("endpoint %s not found", key)
			continue
		}
		if ep.IsPaginated != tc.paginated {
			t.Errorf("%s IsPaginated = %v; want %v", key, ep.IsPaginated, tc.paginated)
		}
	}
}

func TestLoad_PathParams(t *testing.T) {
	endpoints, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	type want struct {
		method string
		path   string
		params []string
	}
	cases := []want{
		{"GET", "/api/tasks/:id", []string{"id"}},
		{"POST", "/api/tasks/:task_id/cancellation", []string{"task_id"}},
		{"PATCH", "/api/tasks/:task_id/todos/:id", []string{"task_id", "id"}},
		{"GET", "/api/tasks", nil},
	}
	epMap := make(map[string]EndpointAttrs, len(endpoints))
	for _, ep := range endpoints {
		epMap[ep.Method+" "+ep.Path] = ep
	}
	for _, tc := range cases {
		key := tc.method + " " + tc.path
		ep, ok := epMap[key]
		if !ok {
			t.Errorf("endpoint %s not found", key)
			continue
		}
		gotSet := make(map[string]bool, len(ep.PathParams))
		for _, p := range ep.PathParams {
			gotSet[p] = true
		}
		for _, want := range tc.params {
			if !gotSet[want] {
				t.Errorf("%s PathParams missing %q; got %v", key, want, ep.PathParams)
			}
		}
		if len(tc.params) == 0 && len(ep.PathParams) != 0 {
			t.Errorf("%s PathParams should be empty; got %v", key, ep.PathParams)
		}
	}
}

func TestCheckQuery_AllowedParam(t *testing.T) {
	endpoints, _ := Load()
	ep := FindEndpoint(endpoints, "GET", "/api/tasks")
	if ep == nil {
		t.Fatal("GET /api/tasks not found")
	}
	req, _ := http.NewRequest(http.MethodGet, "http://x/api/tasks?project_id=p1", nil)
	violations := CheckQuery(req, ep)
	if len(violations) != 0 {
		t.Errorf("expected no violations for documented param; got: %v", violations)
	}
}

func TestCheckQuery_UnknownParam(t *testing.T) {
	endpoints, _ := Load()
	ep := FindEndpoint(endpoints, "GET", "/api/tasks")
	if ep == nil {
		t.Fatal("GET /api/tasks not found")
	}
	req, _ := http.NewRequest(http.MethodGet, "http://x/api/tasks?undocumented_param=x", nil)
	violations := CheckQuery(req, ep)
	if len(violations) == 0 {
		t.Fatal("expected violation for undocumented query param; got none")
	}
	found := false
	for _, v := range violations {
		if strings.Contains(v, "undocumented_param") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'undocumented_param' in violations; got: %v", violations)
	}
}

func TestLoad_OperationID(t *testing.T) {
	endpoints, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	epMap := make(map[string]EndpointAttrs, len(endpoints))
	for _, ep := range endpoints {
		epMap[ep.Method+" "+ep.Path] = ep
	}
	cases := []struct {
		method string
		path   string
		opID   string
	}{
		{"GET", "/api/tasks", "get-api-tasks"},
		{"POST", "/api/tasks", "post-api-tasks"},
		{"POST", "/api/tasks/:task_id/comments", "post-api-task-comments"},
		{"GET", "/api/tasks/:task_id/todos", "get-api-task-todos"},
	}
	for _, tc := range cases {
		key := tc.method + " " + tc.path
		ep, ok := epMap[key]
		if !ok {
			t.Errorf("endpoint %s not found", key)
			continue
		}
		if ep.OperationID != tc.opID {
			t.Errorf("%s OperationID = %q; want %q", key, ep.OperationID, tc.opID)
		}
	}
}

func TestSchemaSourceSync(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("GITHUB_TOKEN not set; skipping source sync check")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, SourceURL, nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.raw+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("fetching source spec: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("source spec responded %d", resp.StatusCode)
	}

	source, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading source spec: %v", err)
	}

	// Normalize both YAML documents to JSON for byte-stable comparison.
	var sourceParsed, localParsed any
	if err := yaml.Unmarshal(source, &sourceParsed); err != nil {
		t.Fatalf("parsing source YAML: %v", err)
	}
	if err := yaml.Unmarshal(fixtureData, &localParsed); err != nil {
		t.Fatalf("parsing local YAML: %v", err)
	}
	sourceNorm, _ := json.Marshal(sourceParsed)
	localNorm, _ := json.Marshal(localParsed)

	if !bytes.Equal(sourceNorm, localNorm) {
		t.Fatal("testdata/api_spec.yaml diverges from the published source spec; update the file to match")
	}
}
