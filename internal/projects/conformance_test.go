package projects

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

func TestConformance_ProjectsList(t *testing.T) {
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
		t.Errorf("conformance violations for projects list: %v", violations)
	}
}

func TestConformance_ProjectsGet(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"projects","id":"p1","attributes":{}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := getCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, []string{"p1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for projects get: %v", violations)
	}
}

func TestConformance_ProjectsCreateNewRepo(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	var gotContentType string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/projects" {
			gotContentType = r.Header.Get("Content-Type")
			violations = schema.CheckRequest(r, endpoints)
			w.WriteHeader(http.StatusCreated)
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"projects","id":"p1","attributes":{}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("name", "MyProject")
	_ = cmd.Flags().Set("platform-id", "pf1")
	_ = cmd.RunE(cmd, nil)

	if len(violations) != 0 {
		t.Errorf("conformance violations for projects create (new): %v", violations)
	}
	if gotContentType != "application/json" {
		t.Errorf("projects create Content-Type = %q; want application/json", gotContentType)
	}
}

func TestConformance_ProjectsCreateExistingRepo(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/projects" {
			violations = schema.CheckRequest(r, endpoints)
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"projects","id":"p1","attributes":{}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("name", "Existing")
	_ = cmd.Flags().Set("platform-id", "pf1")
	_ = cmd.Flags().Set("repo", "https://github.com/org/repo")
	_ = cmd.RunE(cmd, nil)

	if len(violations) != 0 {
		t.Errorf("conformance violations for projects create (existing repo): %v", violations)
	}
}

func TestConformance_ProjectsUpdate(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	var gotContentType string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"projects","id":"19","attributes":{}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("pipeline-id", "9")
	_ = cmd.RunE(cmd, []string{"19"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for projects update: %v", violations)
	}
	if gotContentType != "application/json" {
		t.Errorf("projects update Content-Type = %q; want application/json", gotContentType)
	}
}

func TestConformance_ViolationDetection_ForbiddenTopLevelKeys(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	// Old-style body with forbidden top-level keys.
	body := `{"create_new_repo":"true","new_repo_name":"slug","organization":"org","project":{"display_name":"P","platform_id":"1"}}`
	req, _ := http.NewRequest(http.MethodPost, "http://x/api/projects", strings.NewReader(body))
	violations := schema.CheckRequest(req, endpoints)
	if len(violations) == 0 {
		t.Fatal("expected violations for forbidden top-level keys in projects create; got none")
	}
}

func TestConformance_ProjectsListPaginates(t *testing.T) {
	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.URL.RawQuery == "" {
			_, _ = fmt.Fprintf(w,
				`{"data":[{"type":"projects","id":"p1","attributes":{"name":"Alpha","platform_id":"pf1","git_repo_url":""}}],"links":{"next":"%s/api/projects?page%%5Bnumber%%5D=2"}}`,
				"http://"+r.Host)
		} else {
			_, _ = fmt.Fprint(w,
				`{"data":[{"type":"projects","id":"p2","attributes":{"name":"Beta","platform_id":"pf1","git_repo_url":""}}],"links":{}}`)
		}
	}))
	defer ts.Close()

	var out strings.Builder
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("projects list with pagination failed: %v", err)
	}
	if got := atomic.LoadInt32(&requestCount); got != 2 {
		t.Errorf("expected 2 requests (2 pages); got %d", got)
	}
	result := out.String()
	if !strings.Contains(result, "Alpha") {
		t.Errorf("output missing Alpha from page 1:\n%s", result)
	}
	if !strings.Contains(result, "Beta") {
		t.Errorf("output missing Beta from page 2:\n%s", result)
	}
}
