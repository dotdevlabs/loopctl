package pipelines

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/dotdevlabs/loopctl/internal/schema"
)

func stageFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/pipeline_9.json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestConformance_StagesList(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	fixture := stageFixture(t)
	var document struct {
		Included []struct {
			ID         string
			Type       string
			Attributes map[string]json.RawMessage
		}
	}
	if err := json.Unmarshal(fixture, &document); err != nil {
		t.Fatal(err)
	}
	spec, err := schema.SpecSchema("#/components/schemas/PipelineStageResource", false)
	if err != nil {
		t.Fatal(err)
	}
	var resourceSchema map[string]any
	if err := json.Unmarshal([]byte(spec), &resourceSchema); err != nil {
		t.Fatal(err)
	}
	for _, resource := range document.Included {
		if resource.ID == "unrelated" {
			continue
		}
		validateStageSchema(t, map[string]any{"id": resource.ID, "type": resource.Type, "attributes": decodeValue(t, mustMarshal(t, resource.Attributes))}, resourceSchema)
	}
	for _, jsonMode := range []bool{false, true} {
		t.Run(fmt.Sprint(jsonMode), func(t *testing.T) {
			calls := 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" || r.URL.Path != "/api/pipelines/9" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Accept") != "application/vnd.api+json" || r.Header.Get("Authorization") != "Bearer tok" {
					t.Error("missing request headers")
				}
				if violations := schema.CheckRequest(r, endpoints); len(violations) > 0 {
					t.Errorf("request conformance: %v", violations)
				}
				w.Header().Set("Content-Type", "application/vnd.api+json")
				_, _ = w.Write(fixture)
			}))
			defer ts.Close()
			var out bytes.Buffer
			cmd := stagesListCmd()
			cmd.SetContext(makeCtx(t, ts.URL, "tok", jsonMode, &out))
			if err := cmd.RunE(cmd, []string{"9"}); err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("got %d requests", calls)
			}
			if jsonMode {
				var got []struct {
					ID         string
					Type       string
					Attributes map[string]json.RawMessage
				}
				if err := json.Unmarshal(out.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if len(got) != 12 {
					t.Fatalf("got %d stages", len(got))
				}
				for i, stage := range got {
					if stage.ID != fmt.Sprintf("stage-id-%d", i) || stage.Type != "pipeline_stages" {
						t.Fatalf("wrong resource at %d: %+v", i, stage)
					}
					var expected map[string]json.RawMessage
					for _, resource := range document.Included {
						if resource.ID == stage.ID {
							expected = resource.Attributes
						}
					}
					if !reflect.DeepEqual(decodeValue(t, mustMarshal(t, stage.Attributes)), decodeValue(t, mustMarshal(t, expected))) {
						t.Errorf("attributes changed for %s", stage.ID)
					}
				}
			} else {
				lines := strings.Split(strings.TrimSpace(out.String()), "\n")
				if len(lines) != 14 {
					t.Fatalf("unexpected rows: %s", out.String())
				}
				for _, header := range []string{"ID", "POSITION", "NAME", "ROLE", "STAGE_TYPE", "GATE", "AGENT"} {
					if !strings.Contains(lines[0], header) {
						t.Errorf("missing %s", header)
					}
				}
				for i, line := range lines[2:] {
					fields := strings.Fields(line)
					expected := []string{fmt.Sprintf("stage-id-%d", i), fmt.Sprint(i)}
					if i > 0 {
						expected = append(expected, fmt.Sprintf("stage-%d", i))
					}
					expected = append(expected, "working", "custom", "automated")
					if i > 0 {
						expected = append(expected, fmt.Sprintf("agent-%d", i))
					}
					if !reflect.DeepEqual(fields, expected) {
						t.Errorf("row %d: %v want %v", i, fields, expected)
					}
				}
			}
		})
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func decodeValue(t *testing.T, b []byte) any {
	t.Helper()
	var v any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// Validate the fixture's required keys, published types, nullability and nested shapes.
func validateStageSchema(t *testing.T, v any, s map[string]any) {
	t.Helper()
	if v == nil {
		if s["nullable"] != true {
			t.Error("unexpected null")
		}
		return
	}
	switch s["type"] {
	case "object":
		obj, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("expected object, got %T", v)
		}
		if required, ok := s["required"].([]any); ok {
			for _, key := range required {
				if _, ok := obj[key.(string)]; !ok {
					t.Errorf("missing %s", key)
				}
			}
		}
		props, _ := s["properties"].(map[string]any)
		for key, value := range obj {
			prop, ok := props[key].(map[string]any)
			if !ok {
				if s["additionalProperties"] == true {
					continue
				}
				t.Errorf("unknown key %s", key)
				continue
			}
			validateStageSchema(t, value, prop)
		}
	case "array":
		values, ok := v.([]any)
		if !ok {
			t.Fatalf("expected array, got %T", v)
		}
		for _, value := range values {
			validateStageSchema(t, value, s["items"].(map[string]any))
		}
	case "string":
		if _, ok := v.(string); !ok {
			t.Errorf("expected string, got %T", v)
		}
	case "integer":
		n, ok := v.(json.Number)
		if !ok {
			t.Errorf("expected integer, got %T", v)
		} else if _, err := n.Int64(); err != nil {
			t.Error(err)
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			t.Errorf("expected boolean, got %T", v)
		}
	}
	if enum, ok := s["enum"].([]any); ok {
		found := false
		for _, value := range enum {
			if reflect.DeepEqual(value, v) {
				found = true
			}
		}
		if !found {
			t.Errorf("invalid enum %v", v)
		}
	}
}

func TestStagesList_RelationshipErrorsAndEmpty(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"empty", `{"data":{"relationships":{"stages":{"data":[]}}}}`, ""},
		{"missing", `{"data":{}}`, "relationship"},
		{"null", `{"data":{"relationships":{"stages":{"data":null}}}}`, "array"},
		{"object", `{"data":{"relationships":{"stages":{"data":{}}}}}`, "array"},
		{"bad-id", `{"data":{"relationships":{"stages":{"data":[{"id":3,"type":"pipeline_stages"}]}}}}`, "linkage"},
		{"bad-type", `{"data":{"relationships":{"stages":{"data":[{"id":"s","type":"tasks"}]}}}}`, "invalid"},
		{"missing-resource", `{"data":{"relationships":{"stages":{"data":[{"id":"s","type":"pipeline_stages"}]}}}}`, "missing"},
		{"null-attrs", `{"data":{"relationships":{"stages":{"data":[{"id":"s","type":"pipeline_stages"}]}}},"included":[{"id":"s","type":"pipeline_stages","attributes":null}]}`, "attributes"},
		{"bad-attrs", `{"data":{"relationships":{"stages":{"data":[{"id":"s","type":"pipeline_stages"}]}}},"included":[{"id":"s","type":"pipeline_stages","attributes":{"position":"bad"}}]}`, "decoding stage s"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, jsonMode := range []bool{false, true} {
				calls := 0
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; _, _ = fmt.Fprint(w, tc.body) }))
				var out bytes.Buffer
				cmd := stagesListCmd()
				cmd.SetContext(makeCtx(t, ts.URL, "tok", jsonMode, &out))
				err := cmd.RunE(cmd, []string{"9"})
				ts.Close()
				if tc.want != "" {
					if err == nil || !strings.Contains(err.Error(), tc.want) {
						t.Fatalf("got %v, want %s", err, tc.want)
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					if jsonMode && strings.TrimSpace(out.String()) != "[]" {
						t.Errorf("empty JSON: %s", out.String())
					}
				}
				if calls != 1 {
					t.Errorf("got %d requests", calls)
				}
			}
		})
	}
}
