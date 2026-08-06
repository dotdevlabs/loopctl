package projects

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/config"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

func makeCtx(t *testing.T, serverURL, token string, jsonMode bool, out io.Writer) context.Context {
	t.Helper()
	renderer := output.New(jsonMode, "", out, io.Discard)
	ctx := context.Background()
	ctx = ctxutil.WithRenderer(ctx, renderer)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{JSON: jsonMode})
	ctx = ctxutil.WithActiveContext(ctx, &config.Context{BaseURL: serverURL, Token: token})
	return ctx
}

func TestProjectsList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing/wrong auth header: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Accept") != "application/vnd.api+json" {
			t.Errorf("expected JSON:API Accept header; got: %s", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"projects","id":"p1","attributes":{"name":"Alpha","platform_id":"pf1","git_repo_url":"https://github.com/a/b"}}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "test-token", false, &out)

	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("list failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Alpha") {
		t.Errorf("output missing project name:\n%s", got)
	}
	if !strings.Contains(got, "p1") {
		t.Errorf("output missing project id:\n%s", got)
	}
}

func TestProjectsListJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"projects","id":"p1","attributes":{"name":"Alpha","platform_id":"pf1"}}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", true, &out)

	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("list json failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"data"`) {
		t.Errorf("expected JSON collection envelope; got:\n%s", got)
	}
	if !strings.Contains(got, "Alpha") {
		t.Errorf("expected project name in JSON; got:\n%s", got)
	}
}

func TestProjectsGet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects/p1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"projects","id":"p1","attributes":{"name":"Alpha","platform_id":"pf1"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := getCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"p1"}); err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if !strings.Contains(out.String(), "Alpha") {
		t.Errorf("output missing project name:\n%s", out.String())
	}
}

func TestProjectsGet404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"404","detail":"not found"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := getCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found message; got: %v", err)
	}
}

func TestProjectsCreateNewRepo(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/projects" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"projects","id":"p99","attributes":{"name":"newproj","platform_id":"pf2"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("name", "NewProj")
	_ = cmd.Flags().Set("platform-id", "pf2")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if gotBody["create_new_repo"] != "true" {
		t.Errorf("expected create_new_repo=true; got: %v", gotBody["create_new_repo"])
	}
	if gotBody["new_repo_name"] != "newproj" {
		t.Errorf("expected new_repo_name=newproj; got: %v", gotBody["new_repo_name"])
	}
	if gotBody["organization"] != "dotdevlabs" {
		t.Errorf("expected organization=dotdevlabs; got: %v", gotBody["organization"])
	}
	if gotBody["organization_type"] != "Organization" {
		t.Errorf("expected organization_type=Organization; got: %v", gotBody["organization_type"])
	}
	proj, _ := gotBody["project"].(map[string]any)
	if proj == nil {
		t.Fatal("expected project key in body")
	}
	if proj["display_name"] != "NewProj" {
		t.Errorf("expected display_name=NewProj; got: %v", proj["display_name"])
	}
	if proj["platform_id"] != "pf2" {
		t.Errorf("expected platform_id=pf2; got: %v", proj["platform_id"])
	}
	if !strings.Contains(out.String(), "p99") {
		t.Errorf("expected created id in output; got: %s", out.String())
	}
}

func TestProjectsCreateNewRepoWithPipelineID(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"projects","id":"p100","attributes":{"name":"daybreak","platform_id":"pf1"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("name", "Daybreak")
	_ = cmd.Flags().Set("platform-id", "pf1")
	_ = cmd.Flags().Set("pipeline-id", "pipe42")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	proj, _ := gotBody["project"].(map[string]any)
	if proj == nil {
		t.Fatal("expected project key in body")
	}
	if proj["pipeline_id"] != "pipe42" {
		t.Errorf("expected pipeline_id=pipe42; got: %v", proj["pipeline_id"])
	}
}

func TestProjectsCreateExistingRepo(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"projects","id":"p88","attributes":{"name":"existing","platform_id":"pf3","git_repo_url":"https://github.com/dotdevlabs/existing"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("name", "Existing")
	_ = cmd.Flags().Set("platform-id", "pf3")
	_ = cmd.Flags().Set("repo", "https://github.com/dotdevlabs/existing")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create existing-repo failed: %v", err)
	}

	// Must NOT have create_new_repo.
	if _, ok := gotBody["create_new_repo"]; ok {
		t.Error("existing-repo path must not include create_new_repo key")
	}
	proj, _ := gotBody["project"].(map[string]any)
	if proj == nil {
		t.Fatal("expected project key in body")
	}
	if proj["display_name"] != "Existing" {
		t.Errorf("expected display_name=Existing; got: %v", proj["display_name"])
	}
	if proj["repo"] != "https://github.com/dotdevlabs/existing" {
		t.Errorf("expected repo url; got: %v", proj["repo"])
	}
}

func TestProjectsCreateSlugOverride(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"projects","id":"p77","attributes":{"name":"daybreak-v2","platform_id":"pf1"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("name", "Daybreak")
	_ = cmd.Flags().Set("platform-id", "pf1")
	_ = cmd.Flags().Set("slug", "daybreak-v2")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create slug-override failed: %v", err)
	}

	if gotBody["new_repo_name"] != "daybreak-v2" {
		t.Errorf("expected new_repo_name=daybreak-v2; got: %v", gotBody["new_repo_name"])
	}
}

func TestProjectsCreateSlugDerivation(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"projects","id":"p55","attributes":{"name":"hello-world","platform_id":"pf1"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("name", "Hello World!")
	_ = cmd.Flags().Set("platform-id", "pf1")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create slug-derivation failed: %v", err)
	}

	if gotBody["new_repo_name"] != "hello-world" {
		t.Errorf("expected new_repo_name=hello-world; got: %v", gotBody["new_repo_name"])
	}
}

func TestProjectsCreateAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"422","detail":"slug is taken"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("name", "MyProj")
	_ = cmd.Flags().Set("platform-id", "pf1")

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error on 422")
	}
	if !strings.Contains(err.Error(), "slug is taken") {
		t.Errorf("expected 'slug is taken' in error; got: %v", err)
	}
}

func TestProjectsCreate422Message(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"422","detail":"bad request"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("name", "MyProj")
	_ = cmd.Flags().Set("platform-id", "pf1")

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error on 422")
	}
	if !strings.Contains(err.Error(), "bad request") {
		t.Errorf("expected 'bad request' in error; got: %v", err)
	}
}

func TestProjectsCreateDryRun(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	var out bytes.Buffer
	renderer := output.New(false, "", &out, io.Discard)
	ctx := context.Background()
	ctx = ctxutil.WithRenderer(ctx, renderer)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{DryRun: true})
	ctx = ctxutil.WithActiveContext(ctx, &config.Context{BaseURL: ts.URL, Token: "tok"})

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("name", "DryProj")
	_ = cmd.Flags().Set("platform-id", "pf3")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create dry-run failed: %v", err)
	}
	if called {
		t.Error("dry-run should not make HTTP request")
	}
	got := out.String()
	if !strings.Contains(got, "dry-run") {
		t.Errorf("expected dry-run message; got: %s", got)
	}
	if !strings.Contains(got, "dryproj") {
		t.Errorf("expected derived slug in dry-run message; got: %s", got)
	}
}

func TestProjectsCreateDryRunRepo(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	var out bytes.Buffer
	renderer := output.New(false, "", &out, io.Discard)
	ctx := context.Background()
	ctx = ctxutil.WithRenderer(ctx, renderer)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{DryRun: true})
	ctx = ctxutil.WithActiveContext(ctx, &config.Context{BaseURL: ts.URL, Token: "tok"})

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("name", "ExistingProj")
	_ = cmd.Flags().Set("platform-id", "pf3")
	_ = cmd.Flags().Set("repo", "https://github.com/dotdevlabs/existing")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create dry-run repo failed: %v", err)
	}
	if called {
		t.Error("dry-run should not make HTTP request")
	}
	got := out.String()
	if !strings.Contains(got, "dry-run") {
		t.Errorf("expected dry-run message; got: %s", got)
	}
}

func TestProjectsListVerbose(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"projects","id":"p1","attributes":{"name":"Alpha","platform_id":"pf1"}}]}`)
	}))
	defer ts.Close()

	var out, errBuf bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	ctx = apiclient.WithVerbose(ctx, &errBuf)

	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("list verbose failed: %v", err)
	}

	verbose := errBuf.String()
	if !strings.Contains(verbose, "GET") {
		t.Errorf("expected GET in verbose output; got: %q", verbose)
	}
	if !strings.Contains(verbose, "/api/projects") {
		t.Errorf("expected /api/projects in verbose output; got: %q", verbose)
	}
	if !strings.Contains(verbose, "200") {
		t.Errorf("expected status 200 in verbose output; got: %q", verbose)
	}
}

func TestSlugFromName(t *testing.T) {
	cases := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"Daybreak", "daybreak", false},
		{"My Project", "my-project", false},
		{"Hello World!", "hello-world", false},
		{"My_Project", "my-project", false},
		{"123abc", "abc", false},
		{"---", "", true},
		{"123", "", true},
	}

	for _, tc := range cases {
		got, err := slugFromName(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("slugFromName(%q): expected error, got %q", tc.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("slugFromName(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("slugFromName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
