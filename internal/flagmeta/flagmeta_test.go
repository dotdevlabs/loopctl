package flagmeta

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/airef"
)

func newCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "x"}
	cmd.Flags().String("mode", "", "")
	return cmd
}

func mustPanic(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	f()
}

func TestEnum(t *testing.T) {
	cmd := newCmd()
	Enum(cmd, "mode", "a", "b")
	got := cmd.Flags().Lookup("mode").Annotations[airef.EnumAnnotation]
	if len(got) != 1 || got[0] != `["a","b"]` {
		t.Fatalf("annotation = %v", got)
	}
}

func TestEnumPanics(t *testing.T) {
	mustPanic(t, func() { Enum(newCmd(), "mode") })
	mustPanic(t, func() { Enum(newCmd(), "mode", "a", "a") })
	mustPanic(t, func() { Enum(newCmd(), "missing", "a") })
}

func TestSchema(t *testing.T) {
	cmd := newCmd()
	Schema(cmd, "mode", `{ "type": "object" }`)
	got := cmd.Flags().Lookup("mode").Annotations[airef.SchemaAnnotation]
	if len(got) != 1 || got[0] != `{"type":"object"}` {
		t.Fatalf("annotation = %v", got)
	}
}

func TestSchemaPanics(t *testing.T) {
	mustPanic(t, func() { Schema(newCmd(), "mode", `not json`) })
}

func TestEnumFromSpec(t *testing.T) {
	cmd := newCmd()
	EnumFromSpec(cmd, "mode", "#/components/schemas/PipelineStageInput/properties/gate")
	got := cmd.Flags().Lookup("mode").Annotations[airef.EnumAnnotation]
	if len(got) != 1 || got[0] != `["automated","human"]` {
		t.Fatalf("annotation = %v", got)
	}
}

func TestEnumFromSpecPanics(t *testing.T) {
	mustPanic(t, func() { EnumFromSpec(newCmd(), "mode", "#/components/schemas/Missing/properties/x") })
}

func TestSchemaFromSpec(t *testing.T) {
	cmd := newCmd()
	SchemaFromSpec(cmd, "mode", "#/components/schemas/PipelineStageTrigger", true)
	got := cmd.Flags().Lookup("mode").Annotations[airef.SchemaAnnotation]
	if len(got) != 1 || !strings.HasPrefix(got[0], `{"items":`) || !strings.Contains(got[0], `"type":"array"`) {
		t.Fatalf("annotation = %v", got)
	}
}

func TestSchemaFromSpecPanics(t *testing.T) {
	mustPanic(t, func() { SchemaFromSpec(newCmd(), "mode", "#/components/schemas/Missing", false) })
}
