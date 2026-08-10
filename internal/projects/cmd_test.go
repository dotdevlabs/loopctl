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
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json; got: %s", ct)
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

	// Top-level must only contain "project" — no create_new_repo, new_repo_name, organization, etc.
	if _, ok := gotBody["create_new_repo"]; ok {
		t.Error("body must not contain create_new_repo")
	}
	if _, ok := gotBody["new_repo_name"]; ok {
		t.Error("body must not contain new_repo_name")
	}
	if _, ok := gotBody["organization"]; ok {
		t.Error("body must not contain organization")
	}
	if _, ok := gotBody["organization_type"]; ok {
		t.Error("body must not contain organization_type")
	}
	proj, _ := gotBody["project"].(map[string]any)
	if proj == nil {
		t.Fatal("expected project key in body")
	}
	if proj["display_name"] != "NewProj" {
		t.Errorf("expected display_name=NewProj; got: %v", proj["display_name"])
	}
	if proj["name"] != "newproj" {
		t.Errorf("expected project.name=newproj; got: %v", proj["name"])
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
	if proj["git_repo_url"] != "https://github.com/dotdevlabs/existing" {
		t.Errorf("expected git_repo_url; got: %v", proj["git_repo_url"])
	}
	if _, ok := proj["repo"]; ok {
		t.Error("body must not contain project.repo; use git_repo_url")
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

	proj, _ := gotBody["project"].(map[string]any)
	if proj == nil {
		t.Fatal("expected project key in body")
	}
	if proj["name"] != "daybreak-v2" {
		t.Errorf("expected project.name=daybreak-v2; got: %v", proj["name"])
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

	proj, _ := gotBody["project"].(map[string]any)
	if proj == nil {
		t.Fatal("expected project key in body")
	}
	if proj["name"] != "hello-world" {
		t.Errorf("expected project.name=hello-world; got: %v", proj["name"])
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

func TestProjectsCreateByPlatformName(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		switch r.URL.Path {
		case "/api/platforms":
			if r.Method != http.MethodGet {
				t.Errorf("unexpected method for platforms: %s", r.Method)
			}
			_, _ = fmt.Fprint(w, `{"data":[{"type":"platforms","id":"pf-rails","attributes":{"name":"rails","display_name":"Ruby on Rails"}}]}`)
		case "/api/projects":
			if r.Method != http.MethodPost {
				t.Errorf("unexpected method for projects: %s", r.Method)
			}
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &gotBody)
			_, _ = fmt.Fprint(w, `{"data":{"type":"projects","id":"p200","attributes":{"name":"myapp","platform_id":"pf-rails"}}}`)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("name", "MyApp")
	_ = cmd.Flags().Set("platform", "rails")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create by platform name failed: %v", err)
	}

	proj, _ := gotBody["project"].(map[string]any)
	if proj == nil {
		t.Fatal("expected project key in body")
	}
	if proj["platform_id"] != "pf-rails" {
		t.Errorf("expected resolved platform_id=pf-rails; got: %v", proj["platform_id"])
	}
	if !strings.Contains(out.String(), "p200") {
		t.Errorf("expected created id in output; got: %s", out.String())
	}
}

func TestProjectsCreateByPlatformNameNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.URL.Path == "/api/platforms" {
			_, _ = fmt.Fprint(w, `{"data":[{"type":"platforms","id":"pf1","attributes":{"name":"rails","display_name":"Ruby on Rails"}}]}`)
			return
		}
		t.Errorf("unexpected path called: %s", r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("name", "MyApp")
	_ = cmd.Flags().Set("platform", "django")

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for unknown platform name")
	}
	if !strings.Contains(err.Error(), "django") {
		t.Errorf("expected platform name in error; got: %v", err)
	}
}

func TestProjectsCreateByPipelineName(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		switch r.URL.Path {
		case "/api/platforms":
			_, _ = fmt.Fprint(w, `{"data":[{"type":"platforms","id":"pf1","attributes":{"name":"rails","display_name":"Ruby on Rails"}}]}`)
		case "/api/pipelines":
			if r.Method != http.MethodGet {
				t.Errorf("unexpected method for pipelines: %s", r.Method)
			}
			_, _ = fmt.Fprint(w, `{"data":[{"type":"pipelines","id":"pipe-auto","attributes":{"name":"Autonomous Feature","display_name":"Autonomous Feature"}}]}`)
		case "/api/projects":
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &gotBody)
			_, _ = fmt.Fprint(w, `{"data":{"type":"projects","id":"p300","attributes":{"name":"myapp","platform_id":"pf1"}}}`)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("name", "MyApp")
	_ = cmd.Flags().Set("platform", "rails")
	_ = cmd.Flags().Set("pipeline", "Autonomous Feature")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create by pipeline name failed: %v", err)
	}

	proj, _ := gotBody["project"].(map[string]any)
	if proj == nil {
		t.Fatal("expected project key in body")
	}
	if proj["pipeline_id"] != "pipe-auto" {
		t.Errorf("expected resolved pipeline_id=pipe-auto; got: %v", proj["pipeline_id"])
	}
}

func TestProjectsCreateByPipelineNameNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		switch r.URL.Path {
		case "/api/platforms":
			_, _ = fmt.Fprint(w, `{"data":[{"type":"platforms","id":"pf1","attributes":{"name":"rails","display_name":"Ruby on Rails"}}]}`)
		case "/api/pipelines":
			_, _ = fmt.Fprint(w, `{"data":[{"type":"pipelines","id":"pipe1","attributes":{"name":"autonomous-feature","display_name":"Autonomous Feature"}}]}`)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("name", "MyApp")
	_ = cmd.Flags().Set("platform", "rails")
	_ = cmd.Flags().Set("pipeline", "unknown-pipeline")

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for unknown pipeline name")
	}
	if !strings.Contains(err.Error(), "unknown-pipeline") {
		t.Errorf("expected pipeline name in error; got: %v", err)
	}
}

func TestProjectsCreateNoPlatformError(t *testing.T) {
	var out bytes.Buffer
	ctx := makeCtx(t, "http://localhost", "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("name", "MyApp")

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when no platform selector given")
	}
	if !strings.Contains(err.Error(), "--platform") {
		t.Errorf("expected --platform mention in error; got: %v", err)
	}
}

func TestProjectsCreateBothPlatformFlagsError(t *testing.T) {
	var out bytes.Buffer
	ctx := makeCtx(t, "http://localhost", "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("name", "MyApp")
	_ = cmd.Flags().Set("platform", "rails")
	_ = cmd.Flags().Set("platform-id", "pf1")

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when both --platform and --platform-id given")
	}
	if !strings.Contains(err.Error(), "--platform") {
		t.Errorf("expected --platform mention in error; got: %v", err)
	}
}

func TestProjectsUpdatePipelineID(t *testing.T) {
	var gotBody map[string]any
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"projects","id":"19","attributes":{"name":"myproj","platform_id":"pf1"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "test-token", false, &out)

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("pipeline-id", "9")

	if err := cmd.RunE(cmd, []string{"19"}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("expected PATCH; got %s", gotMethod)
	}
	if gotPath != "/api/projects/19" {
		t.Errorf("expected /api/projects/19; got %s", gotPath)
	}
	proj, _ := gotBody["project"].(map[string]any)
	if proj == nil {
		t.Fatal("expected project key in body")
	}
	if proj["pipeline_id"] != float64(9) {
		t.Errorf("expected pipeline_id=9; got %v", proj["pipeline_id"])
	}
	if !strings.Contains(out.String(), "19") {
		t.Errorf("expected project id in output; got: %s", out.String())
	}
}

func TestProjectsUpdateDisplayName(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"projects","id":"19","attributes":{"name":"myproj","display_name":"New Name","platform_id":"pf1"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("display-name", "New Name")

	if err := cmd.RunE(cmd, []string{"19"}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	proj, _ := gotBody["project"].(map[string]any)
	if proj == nil {
		t.Fatal("expected project key in body")
	}
	if proj["display_name"] != "New Name" {
		t.Errorf("expected display_name=New Name; got %v", proj["display_name"])
	}
	if len(proj) != 1 {
		t.Errorf("expected exactly 1 field in project; got %d: %v", len(proj), proj)
	}
}

func TestProjectsUpdateMultipleFlags(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"projects","id":"19","attributes":{"name":"myproj","platform_id":"pf1"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("git-branch", "main")
	_ = cmd.Flags().Set("display-name", "Foo")

	if err := cmd.RunE(cmd, []string{"19"}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	proj, _ := gotBody["project"].(map[string]any)
	if proj == nil {
		t.Fatal("expected project key in body")
	}
	if proj["git_branch"] != "main" {
		t.Errorf("expected git_branch=main; got %v", proj["git_branch"])
	}
	if proj["display_name"] != "Foo" {
		t.Errorf("expected display_name=Foo; got %v", proj["display_name"])
	}
	if len(proj) != 2 {
		t.Errorf("expected exactly 2 fields in project; got %d: %v", len(proj), proj)
	}
}

func TestProjectsUpdateUnsetFlagsOmitted(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"projects","id":"19","attributes":{"name":"myproj","platform_id":"pf1"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("pipeline-id", "9")

	if err := cmd.RunE(cmd, []string{"19"}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	proj, _ := gotBody["project"].(map[string]any)
	if proj == nil {
		t.Fatal("expected project key in body")
	}
	if len(proj) != 1 {
		t.Errorf("expected exactly 1 field in project; got %d: %v", len(proj), proj)
	}
	for _, unwanted := range []string{"git_branch", "display_name", "environment_id", "container_image", "platform_id", "failure_policy", "fallback_agent_id"} {
		if _, ok := proj[unwanted]; ok {
			t.Errorf("unexpected field %q in project patch body", unwanted)
		}
	}
}

func TestProjectsUpdateNoFlags(t *testing.T) {
	var out bytes.Buffer
	ctx := makeCtx(t, "http://localhost", "tok", false, &out)

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{"19"})
	if err == nil {
		t.Fatal("expected error when no flags set")
	}
	if !strings.Contains(err.Error(), "no fields to update") {
		t.Errorf("expected 'no fields to update' in error; got: %v", err)
	}
}

func TestProjectsUpdate404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"404","detail":"Not Found"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("pipeline-id", "9")

	err := cmd.RunE(cmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not found") {
		t.Errorf("expected 'not found' in error; got: %v", err)
	}
}

func TestProjectsUpdate422(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"422","detail":"pipeline not found"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("pipeline-id", "999")

	err := cmd.RunE(cmd, []string{"19"})
	if err == nil {
		t.Fatal("expected error for 422")
	}
	if !strings.Contains(err.Error(), "pipeline not found") {
		t.Errorf("expected 'pipeline not found' in error; got: %v", err)
	}
}

func TestProjectsUpdateJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"projects","id":"19","attributes":{"name":"myproj","platform_id":"pf1"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", true, &out)

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("pipeline-id", "9")

	if err := cmd.RunE(cmd, []string{"19"}); err != nil {
		t.Fatalf("update json failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"attributes"`) {
		t.Errorf("expected JSON output with 'attributes' key; got:\n%s", got)
	}
	if !strings.Contains(got, "19") {
		t.Errorf("expected project id in JSON output; got:\n%s", got)
	}
}

func TestProjectsUpdateAuth(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"projects","id":"19","attributes":{}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "test-token", false, &out)

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("pipeline-id", "9")

	if err := cmd.RunE(cmd, []string{"19"}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("expected Authorization: Bearer test-token; got: %s", gotAuth)
	}
}

func TestProjectsUpdateContentType(t *testing.T) {
	var gotCT string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"projects","id":"19","attributes":{}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("pipeline-id", "9")

	if err := cmd.RunE(cmd, []string{"19"}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if gotCT != "application/json" {
		t.Errorf("expected Content-Type: application/json; got: %s", gotCT)
	}
}

func TestProjectsUpdateDryRun(t *testing.T) {
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

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("pipeline-id", "9")

	if err := cmd.RunE(cmd, []string{"19"}); err != nil {
		t.Fatalf("update dry-run failed: %v", err)
	}
	if called {
		t.Error("dry-run should not make HTTP request")
	}
	got := out.String()
	if !strings.Contains(got, "dry-run") {
		t.Errorf("expected dry-run message; got: %s", got)
	}
}

func TestProjectsCreateDryRunByPlatformName(t *testing.T) {
	postCalled := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.URL.Path == "/api/platforms" && r.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, `{"data":[{"type":"platforms","id":"pf-rails","attributes":{"name":"rails","display_name":"Ruby on Rails"}}]}`)
			return
		}
		if r.URL.Path == "/api/projects" {
			postCalled = true
		}
		w.WriteHeader(http.StatusInternalServerError)
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
	_ = cmd.Flags().Set("platform", "rails")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create dry-run by platform name failed: %v", err)
	}
	if postCalled {
		t.Error("dry-run should not POST to /api/projects")
	}
	got := out.String()
	if !strings.Contains(got, "dry-run") {
		t.Errorf("expected dry-run message; got: %s", got)
	}
	if !strings.Contains(got, "pf-rails") {
		t.Errorf("expected resolved platform_id in dry-run message; got: %s", got)
	}
}
