package tasks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestTaskStageVersions(t *testing.T) {
	for _, list := range []bool{false, true} {
		for _, jsonMode := range []bool{false, true} {
			for _, version := range []int{0, 42} {
				t.Run(fmt.Sprintf("list=%v/json=%v/version=%d", list, jsonMode, version), func(t *testing.T) {
					calls := 0
					ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						calls++
						if list {
							if r.URL.Query().Get("page") == "2" {
								_, _ = fmt.Fprint(w, `{"data":[{"id":"t2","type":"tasks","attributes":{"kind":"feature","title":"Demo","stage":"implementing","stage_version":7}}]}`)
							} else {
								_, _ = fmt.Fprintf(w, `{"data":[{"id":"t1","type":"tasks","attributes":{"kind":"feature","title":"Demo","stage":"implementing","stage_version":%d}}],"links":{"next":"/api/tasks?page=2"}}`, version)
							}
						} else {
							_, _ = fmt.Fprintf(w, `{"data":{"id":"t1","type":"tasks","attributes":{"kind":"feature","title":"Demo","stage":"implementing","stage_version":%d}}}`, version)
						}
					}))
					defer ts.Close()
					var out bytes.Buffer
					var cmd *cobra.Command
					if list {
						cmd = listCmd()
					} else {
						cmd = getCmd()
					}
					cmd.SetContext(makeCtx(t, ts.URL, "tok", jsonMode, &out))
					if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
						t.Fatal(err)
					}
					wantCalls := 1
					if list {
						wantCalls = 2
					}
					if calls != wantCalls {
						t.Fatalf("calls=%d, want %d", calls, wantCalls)
					}
					if jsonMode {
						var doc struct {
							Data json.RawMessage `json:"data"`
						}
						if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
							t.Fatal(err)
						}
						var resources []struct {
							Attributes map[string]any `json:"attributes"`
						}
						data := doc.Data
						if !list {
							data = append(append([]byte("["), out.Bytes()...), ']')
						}
						if err := json.Unmarshal(data, &resources); err != nil {
							t.Fatal(err)
						}
						if len(resources) != wantCalls {
							t.Fatalf("resources=%d, output=%s", len(resources), out.String())
						}
						if got, ok := resources[0].Attributes["stage_version"].(float64); !ok || got != float64(version) {
							t.Fatalf("wrong version: %s", out.String())
						}
						if list && resources[1].Attributes["stage_version"] != float64(7) {
							t.Fatalf("second page lost version: %s", out.String())
						}
					} else {
						lines := strings.Split(strings.TrimSpace(out.String()), "\n")
						if len(lines) != wantCalls+2 || strings.Fields(lines[0])[4] != "STAGE_VERSION" || strings.Fields(lines[2])[4] != fmt.Sprint(version) {
							t.Fatalf("missing version: %s", out.String())
						}
						if list && strings.Fields(lines[3])[4] != "7" {
							t.Fatalf("second page missing table version: %s", out.String())
						}
					}
				})
			}
		}
	}
}
