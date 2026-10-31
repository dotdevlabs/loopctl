package schema

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSpecEnum(t *testing.T) {
	got, err := SpecEnum("#/components/schemas/TodoAttributes/properties/status")
	if err != nil || !reflect.DeepEqual(got, []string{"pending", "in_progress", "completed"}) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestSpecEnumOneOfUnion(t *testing.T) {
	got, err := SpecEnum("#/components/schemas/FundingEnvelopeProduct/properties/kind")
	if err != nil || !reflect.DeepEqual(got, []string{"trial", "topup", "subscription_prepay"}) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestSpecEnumErrors(t *testing.T) {
	for _, ref := range []string{
		"components/schemas/TodoAttributes",                          // no leading #/
		"#/components/schemas/Missing/properties/status",             // unknown component
		"#/components/schemas/TodoAttributes/properties/content",     // not an enum
		"#/components/schemas/TodoAttributes/properties/status/enum", // not an object
	} {
		if _, err := SpecEnum(ref); err == nil {
			t.Errorf("%s: expected error", ref)
		}
	}
}

func TestSpecSchema(t *testing.T) {
	s, err := SpecSchema("#/components/schemas/PipelineStageTrigger", true)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatal(err)
	}
	items := m["items"].(map[string]any)
	if m["type"] != "array" || items["additionalProperties"] != false {
		t.Fatalf("schema = %s", s)
	}
	if strings.Contains(s, "x-loopcontrol") || strings.Contains(s, "description") {
		t.Fatalf("extensions or descriptions not stripped: %s", s)
	}
	// additionalProperties: true inside config survives as a boolean.
	if !strings.Contains(s, `"config":{"additionalProperties":true,"type":"object"}`) {
		t.Fatalf("config schema lost: %s", s)
	}
}

func TestSpecSchemaResolvesRefs(t *testing.T) {
	s, err := SpecSchema("#/components/schemas/PipelineStageInput", false)
	if err != nil {
		t.Fatal(err)
	}
	// prompt_sections is a $ref to PromptSections, inlined with its keys.
	if !strings.Contains(s, `"stage_introduction"`) || strings.Contains(s, "$ref") {
		t.Fatalf("refs not inlined: %s", s)
	}
}

func TestSpecSchemaErrors(t *testing.T) {
	for _, ref := range []string{
		"bad",
		"#/components/schemas/Missing",
		"#/components/schemas/FundingEnvelopeProduct/oneOf", // not an object
	} {
		if _, err := SpecSchema(ref, false); err == nil {
			t.Errorf("%s: expected error", ref)
		}
	}
	// A oneOf alternative set resolving to several nodes is ambiguous.
	if _, err := SpecSchema("#/components/schemas/FundingEnvelopeProduct/properties/id", false); err == nil {
		t.Error("expected ambiguous error")
	}
}

func withSpec(t *testing.T, doc string) {
	t.Helper()
	old, oldErr := specDoc, specErr
	var n yaml.Node
	if err := yaml.Unmarshal([]byte(doc), &n); err != nil {
		t.Fatal(err)
	}
	specOnce.Do(func() {})
	specDoc, _ = nodeValue(&n).(map[string]any)
	specErr = nil
	t.Cleanup(func() { specDoc, specErr = old, oldErr })
}

func TestSpecRefCycle(t *testing.T) {
	withSpec(t, `
components:
  schemas:
    A:
      $ref: "#/components/schemas/B"
    B:
      $ref: "#/components/schemas/A"
`)
	if _, err := SpecSchema("#/components/schemas/A", false); err == nil || !strings.Contains(err.Error(), "nesting") {
		t.Fatalf("err = %v", err)
	}
	if _, err := SpecEnum("#/components/schemas/A/properties/x"); err == nil {
		t.Fatal("expected cycle error from step")
	}
}

func TestSpecExportShapeErrors(t *testing.T) {
	withSpec(t, `
components:
  schemas:
    BadProp:
      type: object
      properties:
        p: 3
    BadItems:
      type: array
      items: 3
    AddlSchema:
      type: object
      additionalProperties:
        type: string
    BadAddl:
      type: object
      additionalProperties:
        properties:
          q: 1
    DanglingRef:
      type: object
      properties:
        p:
          $ref: "#/components/schemas/Nope"
    ItemErr:
      type: array
      items:
        $ref: "#/components/schemas/Nope"
`)
	for _, c := range []string{"BadProp", "BadItems", "BadAddl", "DanglingRef", "ItemErr"} {
		if _, err := SpecSchema("#/components/schemas/"+c, false); err == nil {
			t.Errorf("%s: expected error", c)
		}
	}
	s, err := SpecSchema("#/components/schemas/AddlSchema", false)
	if err != nil || !strings.Contains(s, `"additionalProperties":{"type":"string"}`) {
		t.Fatalf("%s, %v", s, err)
	}
}

func TestSpecEnumDanglingRefInStep(t *testing.T) {
	withSpec(t, `
components:
  schemas:
    A:
      $ref: "#/components/schemas/Nope"
`)
	if _, err := SpecEnum("#/components/schemas/A/properties/x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSpecEnumRefAndOneOfErrorsInStep(t *testing.T) {
	withSpec(t, `
components:
  schemas:
    R:
      $ref: "#/components/schemas/Target"
    Target:
      properties:
        kind:
          enum: [a, b, a]
    O:
      oneOf:
      - $ref: "#/components/schemas/Nope"
`)
	got, err := SpecEnum("#/components/schemas/R/properties/kind")
	if err != nil || !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("%v, %v", got, err)
	}
	if _, err := SpecEnum("#/components/schemas/O/properties/kind"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSpecRootErrors(t *testing.T) {
	old, oldErr := specDoc, specErr
	t.Cleanup(func() { specDoc, specErr = old, oldErr })
	specErr = yaml.Unmarshal([]byte("a: [1"), &yaml.Node{})
	if _, err := SpecEnum("#/a"); err == nil {
		t.Error("SpecEnum: expected parse error")
	}
	if _, err := SpecSchema("#/a", false); err == nil {
		t.Error("SpecSchema: expected parse error")
	}
}

func TestNodeValue(t *testing.T) {
	var n yaml.Node
	doc := "a: &x [1, 2]\nb: *x\nc: 1\nc: 2\n"
	if err := yaml.Unmarshal([]byte(doc), &n); err != nil {
		t.Fatal(err)
	}
	m := nodeValue(&n).(map[string]any)
	if !reflect.DeepEqual(m["b"], m["a"]) || m["c"] != 1 {
		t.Fatalf("m = %v", m)
	}
}
