package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
	"github.com/dotdevlabs/ctlkit/pkg/config"
	"github.com/dotdevlabs/loopctl/internal/schema"
)

func TestFlatCallerJSONAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[{"status":"404","title":"Not Found","detail":"server detail"}]}`))
	}))
	defer ts.Close()
	_, err := PostJSONFlatErr[map[string]any](context.Background(), &config.Context{BaseURL: ts.URL, Token: "tok"}, "/test", nil)
	cliErr, ok := err.(*clierror.CLIError)
	if !ok || cliErr.Message != "server detail" || cliErr.Code != clierror.CodeNotFound {
		t.Fatalf("unexpected error: %#v", err)
	}
}

func TestAPIErrorContract(t *testing.T) {
	details, err := schema.SpecEnum("#/components/schemas/RecordingContentError/properties/errors/items/properties/detail")
	if err != nil {
		t.Fatal(err)
	}
	for _, detail := range details {
		body, err := json.Marshal(map[string]any{"errors": []map[string]string{{"status": "404", "detail": detail}}})
		if err != nil {
			t.Fatal(err)
		}
		if got := extractAPIError(body, http.StatusNotFound); got != detail {
			t.Errorf("got %q, want %q", got, detail)
		}
	}
	for _, tt := range []struct{ body, want string }{
		{`{"errors":[{"detail":"first"},{"detail":"second"},{"title":"third"}]}`, "first; second; third"},
		{`{"errors":["one","two"]}`, "one; two"},
		{`{"message":"message"}`, "message"},
		{`{"error":"error"}`, "error"},
		{`{"errors":[],"message":"fallback"}`, "fallback"},
		{`{"errors":[{}]}`, `{"errors":[{}]}`},
		{`broken`, "broken"},
		{``, "Not Found"},
		{` `, "Not Found"},
	} {
		if got := extractAPIError([]byte(tt.body), 404); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.body, got, tt.want)
		}
	}
}
