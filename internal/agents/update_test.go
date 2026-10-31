package agents

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"

	"github.com/dotdevlabs/loopctl/internal/schema"
)

const agentResp = `{"data":{"type":"agents","id":"ag1","attributes":{"name":"New","slug":"new","description":null,"install_command":null,"launch_command":"./run.sh","rate_limit_rpm":null,"rate_limit_tpm":null,"credentials_schema":[]},"links":{"self":null}}}`

func runUpdate(t *testing.T, serverURL string, jsonMode bool, out io.Writer, args ...string) error {
	t.Helper()
	cmd := updateCmd()
	cmd.SetContext(makeCtx(t, serverURL, "tok", jsonMode, out))
	cmd.SetOut(out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestAgentsUpdate_RegisteredAndCovered(t *testing.T) {
	// Pre-change: no update subcommand and PATCH /api/agents/:id uncovered.
	c, _, err := NewCmd().Find([]string{"update"})
	if err != nil || c.Name() != "update" {
		t.Fatalf("update subcommand not registered: %v", err)
	}
	if _, ok := schema.Covered[schema.OperationKey{Method: "PATCH", Path: "/api/agents/:id"}]; !ok {
		t.Error("PATCH /api/agents/:id not Covered")
	}
	if _, ok := schema.Excluded[schema.OperationKey{Method: "PUT", Path: "/api/agents/:id"}]; !ok {
		t.Error("PUT /api/agents/:id alias not Excluded")
	}
}

func TestAgentsUpdate_PerFlagBody(t *testing.T) {
	cases := map[string]struct {
		args []string
		key  string
		want any
	}{
		"name":            {[]string{"--name", "N"}, "name", "N"},
		"description":     {[]string{"--description", "D"}, "description", "D"},
		"launch-command":  {[]string{"--launch-command", "go run"}, "launch_command", "go run"},
		"install-command": {[]string{"--install-command", "npm i"}, "install_command", "npm i"},
		"rpm":             {[]string{"--rate-limit-rpm", "50"}, "rate_limit_rpm", float64(50)},
		"tpm":             {[]string{"--rate-limit-tpm", "9000"}, "rate_limit_tpm", float64(9000)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var gotBody map[string]map[string]any
			var gotMethod, gotPath string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				_ = json.NewDecoder(r.Body).Decode(&gotBody)
				w.Header().Set("Content-Type", "application/vnd.api+json")
				_, _ = fmt.Fprint(w, agentResp)
			}))
			defer ts.Close()

			var out bytes.Buffer
			if err := runUpdate(t, ts.URL, false, &out, append([]string{"ag1"}, tc.args...)...); err != nil {
				t.Fatalf("update failed: %v", err)
			}
			if gotMethod != http.MethodPatch || gotPath != "/api/agents/ag1" {
				t.Errorf("unexpected request %s %s", gotMethod, gotPath)
			}
			agent := gotBody["agent"]
			if len(agent) != 1 || agent[tc.key] != tc.want {
				t.Errorf("want only %s=%v; got %v", tc.key, tc.want, gotBody)
			}
			if !strings.Contains(out.String(), "ag1") {
				t.Errorf("output missing id:\n%s", out.String())
			}
		})
	}
}

func TestAgentsUpdate_ExplicitZeroAndEmptySent(t *testing.T) {
	var gotBody map[string]map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, agentResp)
	}))
	defer ts.Close()
	var out bytes.Buffer
	if err := runUpdate(t, ts.URL, false, &out, "ag1", "--install-command", "", "--rate-limit-rpm", "0"); err != nil {
		t.Fatal(err)
	}
	a := gotBody["agent"]
	if v, ok := a["install_command"]; !ok || v != "" {
		t.Errorf("explicit empty install_command not sent: %v", a)
	}
	if v, ok := a["rate_limit_rpm"]; !ok || v != float64(0) {
		t.Errorf("explicit zero rate_limit_rpm not sent: %v", a)
	}
}

func TestAgentsUpdate_NoFlagsUsageError(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { atomic.AddInt32(&calls, 1) }))
	defer ts.Close()
	var out bytes.Buffer
	err := runUpdate(t, ts.URL, false, &out, "ag1")
	if err == nil || !strings.Contains(err.Error(), "no fields to update") {
		t.Fatalf("expected usage error, got %v", err)
	}
	if calls != 0 {
		t.Error("no HTTP call expected")
	}
}

func TestAgentsUpdate_NullableFieldsJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, agentResp)
	}))
	defer ts.Close()
	var out bytes.Buffer
	if err := runUpdate(t, ts.URL, true, &out, "ag1", "--name", "New"); err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(out.Bytes(), &v); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	for _, want := range []string{`"rate_limit_rpm": null`, `"description": null`} {
		if !strings.Contains(strings.ReplaceAll(out.String(), `":null`, `": null`), want) {
			t.Errorf("JSON output missing %s:\n%s", want, out.String())
		}
	}
}

func TestAgentsUpdate_ServerErrors(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusUnprocessableEntity} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(status)
				_, _ = fmt.Fprintf(w, `{"errors":[{"status":"%d","detail":"bad"}]}`, status)
			}))
			defer ts.Close()
			var out bytes.Buffer
			if err := runUpdate(t, ts.URL, false, &out, "ag1", "--launch-command", ""); err == nil {
				t.Fatalf("expected error for %d", status)
			}
		})
	}
}

func TestAgentsUpdate_DryRunNoHTTP(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { atomic.AddInt32(&calls, 1) }))
	defer ts.Close()
	var out bytes.Buffer
	ctx := ctxutil.WithGlobalFlags(makeCtx(t, ts.URL, "tok", false, &out), ctxutil.GlobalFlags{DryRun: true})
	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"ag1", "--name", "x"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if calls != 0 || !strings.Contains(out.String(), "dry-run: would PATCH /api/agents/ag1") {
		t.Errorf("dry-run misbehaved (calls=%d): %s", calls, out.String())
	}
}

func TestConformance_AgentsUpdate(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, agentResp)
	}))
	defer ts.Close()
	var out bytes.Buffer
	err := runUpdate(t, ts.URL, false, &out, "ag1", "--name", "n", "--description", "d",
		"--launch-command", "l", "--install-command", "i", "--rate-limit-rpm", "1", "--rate-limit-tpm", "2")
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Errorf("conformance violations: %v", violations)
	}
}
