package environments

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

func TestEnvironmentsList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/environments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"environments","id":"1","attributes":{"name":"production","description":null,"shared":true,"default":false,"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"}}],"links":{},"meta":{"current_page":1,"page_count":1,"per_page":20,"total_count":1}}`)
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
	if !strings.Contains(got, "production") {
		t.Errorf("output missing environment name:\n%s", got)
	}
	if !strings.Contains(got, "1") {
		t.Errorf("output missing environment id:\n%s", got)
	}
	if !strings.Contains(got, "SHARED") {
		t.Errorf("output missing SHARED column:\n%s", got)
	}
}

func TestEnvironmentsListJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"environments","id":"1","attributes":{"name":"production","description":null,"shared":true,"default":false,"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"}}],"links":{},"meta":{"current_page":1,"page_count":1,"per_page":20,"total_count":1}}`)
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
	if !strings.Contains(got, "production") {
		t.Errorf("expected environment name in JSON; got:\n%s", got)
	}
}

func TestEnvironmentsListError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"500","detail":"internal error"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "internal error") {
		t.Errorf("expected 'internal error' in error; got: %v", err)
	}
}

func TestEnvironmentsGet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/environments/5" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"environments","id":"5","attributes":{"name":"staging","description":"Staging env","shared":false,"default":true,"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := getCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"5"}); err != nil {
		t.Fatalf("get failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "staging") {
		t.Errorf("output missing environment name:\n%s", got)
	}
	if !strings.Contains(got, "DESCRIPTION") {
		t.Errorf("output missing DESCRIPTION column:\n%s", got)
	}
}

func TestEnvironmentsCreate(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/environments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json; got: %s", ct)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":{"type":"environments","id":"10","attributes":{"name":"myenv","description":null,"shared":false,"default":false,"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("name", "myenv")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	env, _ := gotBody["environment"].(map[string]any)
	if env == nil {
		t.Fatal("expected environment key in body")
	}
	if env["name"] != "myenv" {
		t.Errorf("expected name=myenv; got: %v", env["name"])
	}
	if !strings.Contains(out.String(), "10") {
		t.Errorf("expected created id in output; got: %s", out.String())
	}
}

func TestEnvironmentsCreate_DryRun(t *testing.T) {
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
	_ = cmd.Flags().Set("name", "dryenv")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create dry-run failed: %v", err)
	}
	if called {
		t.Error("dry-run should not make HTTP request")
	}
	if !strings.Contains(out.String(), "dry-run") {
		t.Errorf("expected dry-run message; got: %s", out.String())
	}
}

func TestEnvironmentsUpdate(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"environments","id":"5","attributes":{"name":"updated","description":null,"shared":false,"default":false,"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("name", "updated")

	if err := cmd.RunE(cmd, []string{"5"}); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	env, _ := gotBody["environment"].(map[string]any)
	if env == nil {
		t.Fatal("expected environment key in body")
	}
	if env["name"] != "updated" {
		t.Errorf("expected name=updated; got: %v", env["name"])
	}
	if len(env) != 1 {
		t.Errorf("expected exactly 1 field in environment patch; got %d: %v", len(env), env)
	}
}

func TestEnvironmentsUpdate_NoFlags(t *testing.T) {
	var out bytes.Buffer
	ctx := makeCtx(t, "http://localhost", "tok", false, &out)
	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{"5"})
	if err == nil {
		t.Fatal("expected error when no flags set")
	}
	if !strings.Contains(err.Error(), "no fields to update") {
		t.Errorf("expected 'no fields to update' in error; got: %v", err)
	}
}

func TestEnvironmentsEntriesList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/environments/3/entries" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"environment_entries","id":"7","attributes":{"env_var":"MY_VAR","entry_type":"custom","credential_id":null,"credential_name":null,"custom_value":"super-secret-value","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"},"relationships":{"credential_set":{"data":null}},"links":{"self":null}}],"links":{},"meta":{"current_page":1,"page_count":1,"per_page":20,"total_count":1}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := entriesListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"3"}); err != nil {
		t.Fatalf("entries list failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "MY_VAR") {
		t.Errorf("output missing env_var:\n%s", got)
	}
	if !strings.Contains(strings.ToUpper(got), "CREDENTIAL_ID") {
		t.Errorf("output missing CREDENTIAL_ID column:\n%s", got)
	}
	// custom_value must NOT appear in table output
	if strings.Contains(got, "super-secret-value") {
		t.Errorf("output must not contain custom_value (secret leakage):\n%s", got)
	}
	if strings.Contains(strings.ToUpper(got), "CUSTOM_VALUE") {
		t.Errorf("output must not contain CUSTOM_VALUE column:\n%s", got)
	}
}

func TestEnvironmentsEntriesAdd_Custom(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":{"type":"environment_entries","id":"9","attributes":{"env_var":"CUSTOM_VAR","entry_type":"custom","credential_id":null,"credential_name":null,"custom_value":"myvalue","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"},"relationships":{"credential_set":{"data":null}},"links":{"self":null}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := entriesAddCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("env-var", "CUSTOM_VAR")
	_ = cmd.Flags().Set("type", "custom")
	_ = cmd.Flags().Set("value", "myvalue")

	if err := cmd.RunE(cmd, []string{"3"}); err != nil {
		t.Fatalf("entries add custom failed: %v", err)
	}

	entry, _ := gotBody["entry"].(map[string]any)
	if entry == nil {
		t.Fatal("expected entry key in body")
	}
	if entry["env_var"] != "CUSTOM_VAR" {
		t.Errorf("expected env_var=CUSTOM_VAR; got: %v", entry["env_var"])
	}
	if entry["entry_type"] != "custom" {
		t.Errorf("expected entry_type=custom; got: %v", entry["entry_type"])
	}
	if entry["custom_value"] != "myvalue" {
		t.Errorf("expected custom_value=myvalue; got: %v", entry["custom_value"])
	}
}

func TestEnvironmentsEntriesAdd_Stdin(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":{"type":"environment_entries","id":"11","attributes":{"env_var":"STDIN_VAR","entry_type":"custom","credential_id":null,"credential_name":null,"custom_value":"stdin-value","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"},"relationships":{"credential_set":{"data":null}},"links":{"self":null}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := entriesAddCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	cmd.SetIn(bytes.NewBufferString("stdin-value\n"))
	_ = cmd.Flags().Set("env-var", "STDIN_VAR")
	_ = cmd.Flags().Set("type", "custom")
	// --value intentionally NOT set

	if err := cmd.RunE(cmd, []string{"3"}); err != nil {
		t.Fatalf("entries add stdin failed: %v", err)
	}

	entry, _ := gotBody["entry"].(map[string]any)
	if entry == nil {
		t.Fatal("expected entry key in body")
	}
	if entry["custom_value"] != "stdin-value" {
		t.Errorf("expected custom_value=stdin-value from stdin; got: %v", entry["custom_value"])
	}
}

func TestEnvironmentsEntriesAdd_Credential(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":{"type":"environment_entries","id":"12","attributes":{"env_var":"CRED_VAR","entry_type":"credential","credential_id":"42","credential_name":"github-token","custom_value":null,"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"},"relationships":{"credential_set":{"data":{"type":"credential_sets","id":"42"}}},"links":{"self":null}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := entriesAddCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("env-var", "CRED_VAR")
	_ = cmd.Flags().Set("type", "credential")
	_ = cmd.Flags().Set("credential-set-id", "42")

	if err := cmd.RunE(cmd, []string{"3"}); err != nil {
		t.Fatalf("entries add credential failed: %v", err)
	}

	entry, _ := gotBody["entry"].(map[string]any)
	if entry == nil {
		t.Fatal("expected entry key in body")
	}
	if entry["entry_type"] != "credential" {
		t.Errorf("expected entry_type=credential; got: %v", entry["entry_type"])
	}
	if entry["credential_set_id"] != float64(42) {
		t.Errorf("expected credential_set_id=42; got: %v", entry["credential_set_id"])
	}
	if _, ok := entry["service"]; ok {
		t.Errorf("entry body must not contain service key; got: %v", entry)
	}
}

func TestEnvironmentsEntriesChange(t *testing.T) {
	var gotBody map[string]any
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"environment_entries","id":"7","attributes":{"env_var":"UPDATED_VAR","entry_type":"custom","credential_id":null,"credential_name":null,"custom_value":"val","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z"},"relationships":{"credential_set":{"data":null}},"links":{"self":null}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := entriesChangeCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("env-var", "UPDATED_VAR")

	if err := cmd.RunE(cmd, []string{"3", "7"}); err != nil {
		t.Fatalf("entries change failed: %v", err)
	}

	if gotPath != "/api/environments/3/entries/7" {
		t.Errorf("expected path /api/environments/3/entries/7; got: %s", gotPath)
	}
	entry, _ := gotBody["entry"].(map[string]any)
	if entry == nil {
		t.Fatal("expected entry key in body")
	}
	if entry["env_var"] != "UPDATED_VAR" {
		t.Errorf("expected env_var=UPDATED_VAR; got: %v", entry["env_var"])
	}
	if _, ok := entry["service"]; ok {
		t.Errorf("entry body must not contain service key; got: %v", entry)
	}
	if len(entry) != 1 {
		t.Errorf("expected exactly 1 field in entry patch; got %d: %v", len(entry), entry)
	}
}

func TestEnvironmentsEntriesRemove(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := entriesRemoveCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{"3", "7"})
	if err != nil {
		t.Fatalf("entries remove failed: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE; got: %s", gotMethod)
	}
	if gotPath != "/api/environments/3/entries/7" {
		t.Errorf("expected /api/environments/3/entries/7; got: %s", gotPath)
	}
}

func TestEnvironmentsEntriesRemove_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"404","detail":"entry not found"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := entriesRemoveCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{"3", "999"})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "entry not found") {
		t.Errorf("expected 'entry not found' in error; got: %v", err)
	}
}
