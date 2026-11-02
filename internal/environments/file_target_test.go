package environments

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"

	"github.com/dotdevlabs/loopctl/internal/schema"
)

const fileEntryResp = `{"data":{"type":"environment_entries","id":"5","attributes":{"env_var":null,"file_path":"/etc/app/key.pem","encoding":"base64","entry_type":"custom","credential_id":null,"credential_name":null,"custom_value":"x","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"},"relationships":{"credential_set":{"data":null}},"links":{"self":null}}}`

// runEntry runs an entries add/change command against a server that records
// the request body, validates it against the spec and replies with resp.
func runEntry(t *testing.T, mk func() *cobra.Command, flags map[string]string, args []string, status int, resp string, jsonMode, dryRun bool) (body map[string]any, violations []string, out string, err error) {
	t.Helper()
	endpoints := loadSchemaOrSkip(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, resp)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", jsonMode, &buf)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{JSON: jsonMode, DryRun: dryRun})
	cmd := mk()
	cmd.SetContext(ctx)
	cmd.SetOut(&buf)
	for k, v := range flags {
		if e := cmd.Flags().Set(k, v); e != nil {
			t.Fatalf("set --%s: %v", k, e)
		}
	}
	err = cmd.RunE(cmd, args)
	return body, violations, buf.String(), err
}

func entryBody(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	e, _ := body["entry"].(map[string]any)
	if e == nil {
		t.Fatalf("missing entry in body: %v", body)
	}
	return e
}

func TestEntriesAddFileTargetConformance(t *testing.T) {
	for _, enc := range []string{"base64", "raw"} {
		body, v, out, err := runEntry(t, entriesAddCmd, map[string]string{
			"file-path": "/etc/app/key.pem", "encoding": enc, "type": "custom", "value": "x",
		}, []string{"1"}, http.StatusCreated, fileEntryResp, false, false)
		if err != nil {
			t.Fatalf("add %s: %v", enc, err)
		}
		if len(v) != 0 {
			t.Errorf("violations: %v", v)
		}
		e := entryBody(t, body)
		if e["file_path"] != "/etc/app/key.pem" || e["encoding"] != enc {
			t.Errorf("unexpected entry: %v", e)
		}
		if _, ok := e["env_var"]; ok {
			t.Errorf("env_var must not be sent for a file target: %v", e)
		}
		if !strings.Contains(out, "FILE_PATH") || !strings.Contains(out, "/etc/app/key.pem") || !strings.Contains(out, "base64") {
			t.Errorf("output missing file target:\n%s", out)
		}
	}
}

func TestEntriesAddNoTarget(t *testing.T) {
	_, _, _, err := runEntry(t, entriesAddCmd, map[string]string{"type": "custom", "value": "x"},
		[]string{"1"}, http.StatusCreated, fileEntryResp, false, false)
	if err == nil || !strings.Contains(err.Error(), "--env-var or --file-path") {
		t.Fatalf("expected usage error, got %v", err)
	}
}

func TestEntriesAddInvalidEncoding(t *testing.T) {
	_, _, _, err := runEntry(t, entriesAddCmd, map[string]string{
		"file-path": "/x", "encoding": "hex", "type": "custom", "value": "x",
	}, []string{"1"}, http.StatusCreated, fileEntryResp, false, false)
	if err == nil || !strings.Contains(err.Error(), `"raw" or "base64"`) {
		t.Fatalf("expected encoding usage error, got %v", err)
	}
}

func TestEntriesAddBothTargetsSurfacesServerError(t *testing.T) {
	resp := `{"errors":[{"status":"422","title":"Unprocessable","detail":"Env var and file path are mutually exclusive"}]}`
	body, _, _, err := runEntry(t, entriesAddCmd, map[string]string{
		"env-var": "A", "file-path": "/x", "type": "custom", "value": "x",
	}, []string{"1"}, http.StatusUnprocessableEntity, resp, false, false)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected server detail in error, got %v", err)
	}
	e := entryBody(t, body)
	if e["env_var"] != "A" || e["file_path"] != "/x" {
		t.Errorf("client must send both targets: %v", e)
	}
}

func TestEntriesAddDryRunFileTarget(t *testing.T) {
	_, _, out, err := runEntry(t, entriesAddCmd, map[string]string{
		"file-path": "/x", "encoding": "raw", "type": "custom", "value": "x",
	}, []string{"1"}, http.StatusCreated, fileEntryResp, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "dry-run") || !strings.Contains(out, "file_path:/x") || !strings.Contains(out, "encoding:raw") {
		t.Errorf("dry-run output: %s", out)
	}
}

func TestEntriesChangeFileTarget(t *testing.T) {
	cases := []map[string]string{
		{"file-path": "/new/path", "encoding": "raw"},
		{"encoding": "base64"},
		{"file-path": "/only/path"},
	}
	for _, flags := range cases {
		body, v, out, err := runEntry(t, entriesChangeCmd, flags, []string{"1", "5"},
			http.StatusOK, fileEntryResp, false, false)
		if err != nil {
			t.Fatalf("change %v: %v", flags, err)
		}
		if len(v) != 0 {
			t.Errorf("violations: %v", v)
		}
		e := entryBody(t, body)
		if len(e) != len(flags) {
			t.Errorf("patch %v does not match flags %v", e, flags)
		}
		for k, want := range flags {
			key := strings.ReplaceAll(k, "-", "_")
			if e[key] != want {
				t.Errorf("%s = %v, want %s", key, e[key], want)
			}
		}
		if !strings.Contains(out, "/etc/app/key.pem") {
			t.Errorf("output missing file path:\n%s", out)
		}
	}
}

func TestEntriesChangeInvalidEncoding(t *testing.T) {
	_, _, _, err := runEntry(t, entriesChangeCmd, map[string]string{"encoding": "hex"},
		[]string{"1", "5"}, http.StatusOK, fileEntryResp, false, false)
	if err == nil || !strings.Contains(err.Error(), `"raw" or "base64"`) {
		t.Fatalf("expected encoding usage error, got %v", err)
	}
}

func TestEntriesChangeDryRunFileTarget(t *testing.T) {
	_, _, out, err := runEntry(t, entriesChangeCmd, map[string]string{"file-path": "/p", "encoding": "raw"},
		[]string{"1", "5"}, http.StatusOK, fileEntryResp, false, true)
	if err != nil || !strings.Contains(out, "file_path:/p") || !strings.Contains(out, "encoding:raw") {
		t.Errorf("dry-run: err=%v out=%s", err, out)
	}
}

func TestEntriesChangeNoFieldHint(t *testing.T) {
	_, _, _, err := runEntry(t, entriesChangeCmd, nil, []string{"1", "5"}, http.StatusOK, fileEntryResp, false, false)
	if err == nil || !strings.Contains(err.Error(), "no fields to update") {
		t.Fatalf("got %v", err)
	}
}

func TestEntriesAddJSONIncludesFileTarget(t *testing.T) {
	_, _, out, err := runEntry(t, entriesAddCmd, map[string]string{
		"file-path": "/etc/app/key.pem", "encoding": "base64", "type": "custom", "value": "x",
	}, []string{"1"}, http.StatusCreated, fileEntryResp, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"file_path"`) || !strings.Contains(out, `"encoding"`) {
		t.Errorf("json output missing attributes:\n%s", out)
	}
}

func TestEntriesListShowsFileTarget(t *testing.T) {
	list := `{"data":[` + strings.TrimSuffix(strings.TrimPrefix(fileEntryResp, `{"data":`), `}`) +
		`,{"type":"environment_entries","id":"6","attributes":{"env_var":"PLAIN","file_path":null,"encoding":null,"entry_type":"custom","credential_id":null,"credential_name":null,"custom_value":"v","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"}}],"links":{},"meta":{"current_page":1,"page_count":1,"per_page":20,"total_count":2}}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, list)
	}))
	defer ts.Close()
	var out bytes.Buffer
	cmd := entriesListCmd()
	cmd.SetContext(makeCtx(t, ts.URL, "tok", false, &out))
	cmd.SetOut(&out)
	if err := cmd.RunE(cmd, []string{"3"}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"FILE_PATH", "ENCODING", "/etc/app/key.pem", "base64", "PLAIN"} {
		if !strings.Contains(got, want) {
			t.Errorf("list output missing %q:\n%s", want, got)
		}
	}
}
