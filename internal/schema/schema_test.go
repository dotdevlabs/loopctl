package schema

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
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
		"POST /api/task_kinds",
	} {
		if !found[want] {
			t.Errorf("Load() missing endpoint %q", want)
		}
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
	// POST /api/tasks/:task_id/cancellation has request_body null — no body is correct.
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
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("fetching source contract: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("source contract responded %d", resp.StatusCode)
	}

	source, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading source contract: %v", err)
	}

	var sourceParsed, localParsed any
	if err := json.Unmarshal(source, &sourceParsed); err != nil {
		t.Fatalf("parsing source: %v", err)
	}
	if err := json.Unmarshal(fixtureData, &localParsed); err != nil {
		t.Fatalf("parsing local copy: %v", err)
	}
	sourceNorm, _ := json.Marshal(sourceParsed)
	localNorm, _ := json.Marshal(localParsed)

	if !bytes.Equal(sourceNorm, localNorm) {
		t.Fatal("testdata/schema.json diverges from the published source document; update the file to match")
	}
}
