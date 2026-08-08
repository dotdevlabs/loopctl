package tasks

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

func TestConformance_TasksList(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[]}`)
	}))
	defer ts.Close()

	var buf io.Writer = io.Discard
	ctx := makeCtx(t, ts.URL, "tok", false, buf)
	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("project-id", "proj1")
	_ = cmd.RunE(cmd, nil)

	if len(violations) != 0 {
		t.Errorf("conformance violations for tasks list: %v", violations)
	}
}

func TestConformance_TasksGet(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"t1","attributes":{}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := getCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, []string{"t1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for tasks get: %v", violations)
	}
}

func TestConformance_TasksCreate(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	var gotContentType string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"t1","attributes":{}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("project-id", "proj1")
	_ = cmd.Flags().Set("kind", "feature")
	_ = cmd.Flags().Set("title", "T")
	_ = cmd.Flags().Set("description", "D")
	_ = cmd.RunE(cmd, nil)

	if len(violations) != 0 {
		t.Errorf("conformance violations for tasks create: %v", violations)
	}
	if gotContentType != "application/json" {
		t.Errorf("tasks create Content-Type = %q; want application/json", gotContentType)
	}
}

func TestConformance_TasksUpdate(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"t1","attributes":{}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("implementation-criteria", "Do the thing")
	_ = cmd.RunE(cmd, []string{"t1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for tasks update: %v", violations)
	}
}

func TestConformance_TasksCancel(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"status":"cancelled","stage":"rejected"}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := cancelCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, []string{"t1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for tasks cancel: %v", violations)
	}
}

func TestConformance_TasksWatch(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var taskViolations, activitiesViolations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		switch r.URL.Path {
		case "/api/tasks/t1":
			taskViolations = schema.CheckRequest(r, endpoints)
			_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"t1","attributes":{"stage":"completed","pr_number":0}}}`)
		case "/api/tasks/t1/activities":
			activitiesViolations = schema.CheckRequest(r, endpoints)
			_, _ = fmt.Fprint(w, `{"data":[]}`)
		}
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := watchCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("interval", "10ms")
	_ = cmd.RunE(cmd, []string{"t1"})

	if len(taskViolations) != 0 {
		t.Errorf("conformance violations for tasks watch (task GET): %v", taskViolations)
	}
	if len(activitiesViolations) != 0 {
		t.Errorf("conformance violations for tasks watch (activities GET): %v", activitiesViolations)
	}
}

func TestConformance_ViolationDetection(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	// Verify that CheckRequest detects a forbidden field in tasks create.
	body := `{"task":{"project_id":"p1","kind":"feature","title":"T","description":"D","FORBIDDEN":"x"}}`
	req, _ := http.NewRequest(http.MethodPost, "http://x/api/tasks", strings.NewReader(body))
	violations := schema.CheckRequest(req, endpoints)
	if len(violations) == 0 {
		t.Fatal("expected violation for FORBIDDEN field in tasks create; got none")
	}
}
