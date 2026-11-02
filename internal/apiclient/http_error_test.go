package apiclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGETStatusAndBody(t *testing.T) {
	for _, body := range []string{`{"errors":[{"detail":"specific detail"}]}`, `{"message":"specific detail"}`, `{"error":"specific detail"}`, "specific detail", ""} {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404); _, _ = io.WriteString(w, body) }))
		_, err := GetJSONAPISingleFull[struct{}](context.Background(), testActiveCtx(ts.URL), "/test")
		var status *HTTPError
		if !errors.As(err, &status) || status.StatusCode != 404 || errors.Unwrap(status) == nil {
			t.Fatalf("%v", err)
		}
		want := "specific detail"
		if body == "" {
			want = "Not Found"
		}
		if !strings.Contains(err.Error(), want) {
			t.Fatal(err)
		}
		ts.Close()
	}
}
