package tasks

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
	var gotContentType string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
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
	if gotContentType != "application/json" {
		t.Errorf("tasks update Content-Type = %q; want application/json", gotContentType)
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

func TestConformance_TasksCreate_WithDependsOn(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	_ = cmd.Flags().Set("depends-on", "dep1")
	_ = cmd.RunE(cmd, nil)

	if len(violations) != 0 {
		t.Errorf("conformance violations for tasks create with depends_on: %v", violations)
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

func TestConformance_TasksListPaginates(t *testing.T) {
	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.URL.RawQuery == "project_id=proj1" {
			// First page: has a next link pointing to page 2.
			_, _ = fmt.Fprintf(w,
				`{"data":[{"type":"tasks","id":"t1","attributes":{"kind":"feature","title":"Task One","stage":"planning","status":"open"}}],"links":{"next":"%s/api/tasks?project_id=proj1&page%%5Bnumber%%5D=2"}}`,
				"http://"+r.Host)
		} else {
			// Second page: no next link.
			_ = n
			_, _ = fmt.Fprint(w,
				`{"data":[{"type":"tasks","id":"t2","attributes":{"kind":"feature","title":"Task Two","stage":"implementing","status":"open"}}],"links":{}}`)
		}
	}))
	defer ts.Close()

	var out strings.Builder
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("project-id", "proj1")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("list with pagination failed: %v", err)
	}
	if got := atomic.LoadInt32(&requestCount); got != 2 {
		t.Errorf("expected 2 requests (2 pages); got %d", got)
	}
	result := out.String()
	if !strings.Contains(result, "Task One") {
		t.Errorf("output missing Task One from page 1:\n%s", result)
	}
	if !strings.Contains(result, "Task Two") {
		t.Errorf("output missing Task Two from page 2:\n%s", result)
	}
}

func TestConformance_TasksListGlobal(t *testing.T) {
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
	// no --project-id: global account listing
	_ = cmd.RunE(cmd, nil)

	if len(violations) != 0 {
		t.Errorf("conformance violations for global tasks list: %v", violations)
	}
}

func TestConformance_TasksListGlobalPaginates(t *testing.T) {
	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.URL.RawQuery == "" {
			// First page: no query string, return next link.
			_, _ = fmt.Fprintf(w,
				`{"data":[{"type":"tasks","id":"t1","attributes":{"kind":"feature","title":"Global One","stage":"planning","status":"open"}}],"links":{"next":"%s/api/tasks?page%%5Bnumber%%5D=2"}}`,
				"http://"+r.Host)
		} else {
			// Second page.
			_ = n
			_, _ = fmt.Fprint(w,
				`{"data":[{"type":"tasks","id":"t2","attributes":{"kind":"feature","title":"Global Two","stage":"implementing","status":"open"}}],"links":{}}`)
		}
	}))
	defer ts.Close()

	var out strings.Builder
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	// no --project-id

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("global list with pagination failed: %v", err)
	}
	if got := atomic.LoadInt32(&requestCount); got != 2 {
		t.Errorf("expected 2 requests (2 pages); got %d", got)
	}
	result := out.String()
	if !strings.Contains(result, "Global One") {
		t.Errorf("output missing Global One from page 1:\n%s", result)
	}
	if !strings.Contains(result, "Global Two") {
		t.Errorf("output missing Global Two from page 2:\n%s", result)
	}
}

func TestConformance_TasksUnblock(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":1,"status":"open","stage":"implementing"}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := unblockCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, []string{"t1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for tasks unblock: %v", violations)
	}
}

func TestConformance_TasksWatchUsesLinksself(t *testing.T) {
	var requestedPaths []string
	var taskCallIdx int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		switch r.URL.Path {
		case "/api/tasks/t1":
			// First call: respond with links.self pointing to the canonical URL.
			n := atomic.AddInt32(&taskCallIdx, 1)
			if n == 1 {
				_, _ = fmt.Fprintf(w,
					`{"data":{"type":"tasks","id":"t1","links":{"self":"/api/tasks/t1-canonical"},"attributes":{"stage":"planning","pr_number":0}}}`)
			} else {
				_, _ = fmt.Fprint(w,
					`{"data":{"type":"tasks","id":"t1","attributes":{"stage":"completed","pr_number":0}}}`)
			}
		case "/api/tasks/t1-canonical":
			_, _ = fmt.Fprint(w,
				`{"data":{"type":"tasks","id":"t1","links":{"self":"/api/tasks/t1-canonical"},"attributes":{"stage":"completed","pr_number":0}}}`)
		case "/api/tasks/t1/activities", "/api/tasks/t1-canonical/activities":
			_, _ = fmt.Fprint(w, `{"data":[]}`)
		}
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := watchCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("interval", "10ms")

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("watch with links.self failed: %v", err)
	}

	// The first task GET should use the constructed path /api/tasks/t1.
	// All subsequent task GETs should use the self-link /api/tasks/t1-canonical.
	taskPaths := make([]string, 0)
	for _, p := range requestedPaths {
		if strings.HasPrefix(p, "/api/tasks/") && !strings.HasSuffix(p, "/activities") {
			taskPaths = append(taskPaths, p)
		}
	}
	if len(taskPaths) < 2 {
		t.Fatalf("expected at least 2 task GETs; got %v", taskPaths)
	}
	if taskPaths[0] != "/api/tasks/t1" {
		t.Errorf("first task GET should be /api/tasks/t1; got %q", taskPaths[0])
	}
	for _, p := range taskPaths[1:] {
		if p != "/api/tasks/t1-canonical" {
			t.Errorf("subsequent task GETs should use links.self /api/tasks/t1-canonical; got %q", p)
		}
	}
}
