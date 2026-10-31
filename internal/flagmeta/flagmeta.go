// Package flagmeta declares machine-readable flag metadata (enums and JSON
// schemas) that ctlkit surfaces through "ai --json".
package flagmeta

import (
	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/airef"
	"github.com/dotdevlabs/loopctl/internal/schema"
)

// Enum declares the fixed set of accepted values for the named flag of cmd.
// The values are static literals, so a failure is a programming error and panics.
func Enum(cmd *cobra.Command, flag string, values ...string) {
	if err := airef.SetFlagEnum(cmd, flag, values); err != nil {
		panic("flagmeta: " + err.Error())
	}
}

// Schema declares the JSON Schema of the named JSON-valued flag of cmd.
// The schema is a static literal, so a failure is a programming error and panics.
func Schema(cmd *cobra.Command, flag, schemaJSON string) {
	if err := airef.SetFlagSchema(cmd, flag, schemaJSON); err != nil {
		panic("flagmeta: " + err.Error())
	}
}

// EnumFromSpec declares the enum of the named flag of cmd from the JSON pointer
// ref into the embedded API spec copy, so the flag cannot drift from the spec.
// An unresolvable ref is a programming error and panics.
func EnumFromSpec(cmd *cobra.Command, flag, ref string) {
	values, err := schema.SpecEnum(ref)
	if err != nil {
		panic("flagmeta: " + err.Error())
	}
	Enum(cmd, flag, values...)
}

// SchemaFromSpec declares the JSON Schema of the named flag of cmd from the
// JSON pointer ref into the embedded API spec copy; array wraps it as an array
// of that schema. An unresolvable ref is a programming error and panics.
func SchemaFromSpec(cmd *cobra.Command, flag, ref string, array bool) {
	s, err := schema.SpecSchema(ref, array)
	if err != nil {
		panic("flagmeta: " + err.Error())
	}
	Schema(cmd, flag, s)
}
