package pipelines

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// StageInput holds all pipeline stage attributes per the published API contract.
// All fields except Name use omitempty so only explicitly-set attributes are sent.
type StageInput struct {
	Name                string          `json:"name"`
	Role                string          `json:"role,omitempty"`
	StageType           string          `json:"stage_type,omitempty"`
	CustomStageName     string          `json:"custom_stage_name,omitempty"`
	Template            string          `json:"template,omitempty"`
	Instructions        string          `json:"instructions,omitempty"`
	Gate                string          `json:"gate,omitempty"`
	Agent               string          `json:"agent,omitempty"`
	AdvanceNotice       string          `json:"advance_notice,omitempty"`
	RunsInContainer     *bool           `json:"runs_in_container,omitempty"`
	Position            *int            `json:"position,omitempty"`
	OnFailure           *OnFailureAttrs `json:"on_failure,omitempty"`
	PromptSections      json.RawMessage `json:"prompt_sections,omitempty"`
	StageTriggers       json.RawMessage `json:"stage_triggers,omitempty"`
	AdvanceRequirements json.RawMessage `json:"advance_requirements,omitempty"`
	BranchConditions    json.RawMessage `json:"branch_conditions,omitempty"`
	Environment         json.RawMessage `json:"environment,omitempty"`
}

// OnFailureAttrs holds the on_failure sub-object attributes.
type OnFailureAttrs struct {
	MaxReworkCount   int  `json:"max_rework_count,omitempty"`
	ReworkToPosition *int `json:"rework_to_position,omitempty"`
}

// stageDisplayAttrs is used for table rendering of a stage.
type stageDisplayAttrs struct {
	Position  int    `json:"position"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	StageType string `json:"stage_type"`
	Gate      string `json:"gate"`
	Agent     string `json:"agent"`
}

func stageCols() []output.Column {
	return []output.Column{
		{Header: "POSITION"},
		{Header: "NAME"},
		{Header: "ROLE"},
		{Header: "STAGE_TYPE"},
		{Header: "GATE"},
		{Header: "AGENT"},
	}
}

// stagesCmd returns the "stages" subcommand under "pipelines".
func stagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stages",
		Short: "Manage pipeline stages",
	}
	cmd.AddCommand(stagesListCmd())
	cmd.AddCommand(stagesAddCmd())
	cmd.AddCommand(stagesUpdateCmd())
	cmd.AddCommand(stagesRemoveCmd())
	return cmd
}

func stagesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <pipeline-id>",
		Short: "List stages for a pipeline",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id := args[0]
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			res, err := apiclient.GetJSONAPISingle[PipelineAttrs](ctx, activeCtx, "/api/pipelines/"+id)
			if err != nil {
				return err
			}

			cols := stageCols()
			rows := make([][]string, 0, len(res.Attributes.Stages))
			for i, raw := range res.Attributes.Stages {
				var attrs stageDisplayAttrs
				_ = json.Unmarshal(raw, &attrs)
				pos := fmt.Sprintf("%d", i)
				if attrs.Position != 0 {
					pos = fmt.Sprintf("%d", attrs.Position)
				}
				rows = append(rows, []string{pos, attrs.Name, attrs.Role, attrs.StageType, attrs.Gate, attrs.Agent})
			}
			return r.Render(cols, rows, res)
		},
	}
}

func stagesAddCmd() *cobra.Command {
	var (
		name               string
		role               string
		stageType          string
		customStageName    string
		template           string
		instructions       string
		gate               string
		agent              string
		advanceNotice      string
		position           int
		runsInContainer    bool
		onFailureJSON      string
		promptSectionsJSON string
		stageTriggersJSON  string
		advanceReqJSON     string
		branchCondJSON     string
		environmentJSON    string
	)

	cmd := &cobra.Command{
		Use:   "add <pipeline-id>",
		Short: "Add a stage to a pipeline",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id := args[0]

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would PATCH /api/pipelines/%s to append stage %q\n", id, name)
				return nil
			}

			// Build StageInput from flags.
			stage := StageInput{Name: name}
			if cmd.Flags().Changed("role") {
				stage.Role = role
			}
			if cmd.Flags().Changed("stage-type") {
				stage.StageType = stageType
			}
			if cmd.Flags().Changed("custom-stage-name") {
				stage.CustomStageName = customStageName
			}
			if cmd.Flags().Changed("template") {
				stage.Template = template
			}
			if cmd.Flags().Changed("instructions") {
				stage.Instructions = instructions
			}
			if cmd.Flags().Changed("gate") {
				stage.Gate = gate
			}
			if cmd.Flags().Changed("agent") {
				stage.Agent = agent
			}
			if cmd.Flags().Changed("advance-notice") {
				stage.AdvanceNotice = advanceNotice
			}
			if cmd.Flags().Changed("position") {
				p := position
				stage.Position = &p
			}
			if cmd.Flags().Changed("runs-in-container") {
				b := runsInContainer
				stage.RunsInContainer = &b
			}

			// Validate and set JSON fields.
			if cmd.Flags().Changed("on-failure") {
				var of OnFailureAttrs
				if err := json.Unmarshal([]byte(onFailureJSON), &of); err != nil {
					return fmt.Errorf("--on-failure: invalid JSON: %w", err)
				}
				stage.OnFailure = &of
			}
			if cmd.Flags().Changed("prompt-sections") {
				raw, err := validateJSON(promptSectionsJSON, "--prompt-sections")
				if err != nil {
					return err
				}
				stage.PromptSections = raw
			}
			if cmd.Flags().Changed("stage-triggers") {
				raw, err := validateJSON(stageTriggersJSON, "--stage-triggers")
				if err != nil {
					return err
				}
				stage.StageTriggers = raw
			}
			if cmd.Flags().Changed("advance-requirements") {
				raw, err := validateJSON(advanceReqJSON, "--advance-requirements")
				if err != nil {
					return err
				}
				stage.AdvanceRequirements = raw
			}
			if cmd.Flags().Changed("branch-conditions") {
				raw, err := validateJSON(branchCondJSON, "--branch-conditions")
				if err != nil {
					return err
				}
				stage.BranchConditions = raw
			}
			if cmd.Flags().Changed("environment") {
				raw, err := validateJSON(environmentJSON, "--environment")
				if err != nil {
					return err
				}
				stage.Environment = raw
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			// Read current stages.
			pipeline, err := apiclient.GetJSONAPISingle[PipelineAttrs](ctx, activeCtx, "/api/pipelines/"+id)
			if err != nil {
				return err
			}

			stageRaw, err := json.Marshal(stage)
			if err != nil {
				return fmt.Errorf("marshaling stage: %w", err)
			}
			stages := append(pipeline.Attributes.Stages, json.RawMessage(stageRaw))

			body := map[string]any{"pipeline": map[string]any{"stages": stages}}
			res, err := apiclient.PatchJSONBodyJSONAPIResponse[PipelineAttrs](ctx, activeCtx, "/api/pipelines/"+id, body)
			if err != nil {
				return err
			}

			p := res.Attributes
			rows := [][]string{{res.ID, p.Name, p.Kind}}
			return r.Render(pipelineCols(), rows, res)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Stage name (required)")
	cmd.Flags().StringVar(&role, "role", "", "Stage role (planning, implementing, reviewing, etc.)")
	cmd.Flags().StringVar(&stageType, "stage-type", "", "stage_type for custom stage types")
	cmd.Flags().StringVar(&customStageName, "custom-stage-name", "", "custom_stage_name (custom type only)")
	cmd.Flags().StringVar(&template, "template", "", "Template (custom type only)")
	cmd.Flags().StringVar(&instructions, "instructions", "", "Instructions / prompt text")
	cmd.Flags().StringVar(&gate, "gate", "", "Advance gate (e.g. manual, ci_pass)")
	cmd.Flags().StringVar(&agent, "agent", "", "Agent slug override")
	cmd.Flags().StringVar(&advanceNotice, "advance-notice", "", "Advance notice value")
	cmd.Flags().IntVar(&position, "position", 0, "Explicit position override")
	cmd.Flags().BoolVar(&runsInContainer, "runs-in-container", false, "runs_in_container override")
	cmd.Flags().StringVar(&onFailureJSON, "on-failure", "", `JSON: {"max_rework_count":3,"rework_to_position":0}`)
	cmd.Flags().StringVar(&promptSectionsJSON, "prompt-sections", "", "JSON array of prompt section objects")
	cmd.Flags().StringVar(&stageTriggersJSON, "stage-triggers", "", "JSON array of trigger strings")
	cmd.Flags().StringVar(&advanceReqJSON, "advance-requirements", "", "JSON array of requirement strings")
	cmd.Flags().StringVar(&branchCondJSON, "branch-conditions", "", "JSON array of branch condition strings")
	cmd.Flags().StringVar(&environmentJSON, "environment", "", "JSON object of environment key/value pairs")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func stagesUpdateCmd() *cobra.Command {
	var (
		role               string
		stageType          string
		customStageName    string
		template           string
		instructions       string
		gate               string
		agent              string
		advanceNotice      string
		position           int
		runsInContainer    bool
		onFailureJSON      string
		promptSectionsJSON string
		stageTriggersJSON  string
		advanceReqJSON     string
		branchCondJSON     string
		environmentJSON    string
	)

	cmd := &cobra.Command{
		Use:   "update <pipeline-id> <stage-name>",
		Short: "Update a stage in a pipeline",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id := args[0]
			stageName := args[1]

			// Validate at least one flag changed before any HTTP call.
			flagNames := []string{
				"role", "stage-type", "custom-stage-name", "template", "instructions",
				"gate", "agent", "advance-notice", "position", "runs-in-container",
				"on-failure", "prompt-sections", "stage-triggers", "advance-requirements",
				"branch-conditions", "environment",
			}
			anyChanged := false
			for _, f := range flagNames {
				if cmd.Flags().Changed(f) {
					anyChanged = true
					break
				}
			}
			if !anyChanged {
				return fmt.Errorf("at least one flag must be provided")
			}

			// Validate JSON flags before any HTTP call.
			if cmd.Flags().Changed("on-failure") {
				var of OnFailureAttrs
				if err := json.Unmarshal([]byte(onFailureJSON), &of); err != nil {
					return fmt.Errorf("--on-failure: invalid JSON: %w", err)
				}
			}
			for flagName, val := range map[string]string{
				"prompt-sections":      promptSectionsJSON,
				"stage-triggers":       stageTriggersJSON,
				"advance-requirements": advanceReqJSON,
				"branch-conditions":    branchCondJSON,
				"environment":          environmentJSON,
			} {
				if cmd.Flags().Changed(flagName) {
					if _, err := validateJSON(val, "--"+flagName); err != nil {
						return err
					}
				}
			}

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would PATCH /api/pipelines/%s to update stage %q\n", id, stageName)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			// Read current stages.
			pipeline, err := apiclient.GetJSONAPISingle[PipelineAttrs](ctx, activeCtx, "/api/pipelines/"+id)
			if err != nil {
				return err
			}

			// Find and update the stage by name.
			found := false
			updated := make([]json.RawMessage, len(pipeline.Attributes.Stages))
			for i, raw := range pipeline.Attributes.Stages {
				var stageMap map[string]any
				if err := json.Unmarshal(raw, &stageMap); err != nil {
					return fmt.Errorf("decoding stage %d: %w", i, err)
				}
				nameVal, _ := stageMap["name"].(string)
				if nameVal != stageName {
					updated[i] = raw
					continue
				}
				found = true

				// Apply only changed flags.
				if cmd.Flags().Changed("role") {
					stageMap["role"] = role
				}
				if cmd.Flags().Changed("stage-type") {
					stageMap["stage_type"] = stageType
				}
				if cmd.Flags().Changed("custom-stage-name") {
					stageMap["custom_stage_name"] = customStageName
				}
				if cmd.Flags().Changed("template") {
					stageMap["template"] = template
				}
				if cmd.Flags().Changed("instructions") {
					stageMap["instructions"] = instructions
				}
				if cmd.Flags().Changed("gate") {
					stageMap["gate"] = gate
				}
				if cmd.Flags().Changed("agent") {
					stageMap["agent"] = agent
				}
				if cmd.Flags().Changed("advance-notice") {
					stageMap["advance_notice"] = advanceNotice
				}
				if cmd.Flags().Changed("position") {
					stageMap["position"] = position
				}
				if cmd.Flags().Changed("runs-in-container") {
					stageMap["runs_in_container"] = runsInContainer
				}
				if cmd.Flags().Changed("on-failure") {
					var of OnFailureAttrs
					_ = json.Unmarshal([]byte(onFailureJSON), &of)
					stageMap["on_failure"] = of
				}
				if cmd.Flags().Changed("prompt-sections") {
					var v any
					_ = json.Unmarshal([]byte(promptSectionsJSON), &v)
					stageMap["prompt_sections"] = v
				}
				if cmd.Flags().Changed("stage-triggers") {
					var v any
					_ = json.Unmarshal([]byte(stageTriggersJSON), &v)
					stageMap["stage_triggers"] = v
				}
				if cmd.Flags().Changed("advance-requirements") {
					var v any
					_ = json.Unmarshal([]byte(advanceReqJSON), &v)
					stageMap["advance_requirements"] = v
				}
				if cmd.Flags().Changed("branch-conditions") {
					var v any
					_ = json.Unmarshal([]byte(branchCondJSON), &v)
					stageMap["branch_conditions"] = v
				}
				if cmd.Flags().Changed("environment") {
					var v any
					_ = json.Unmarshal([]byte(environmentJSON), &v)
					stageMap["environment"] = v
				}

				newRaw, err := json.Marshal(stageMap)
				if err != nil {
					return fmt.Errorf("re-encoding stage: %w", err)
				}
				updated[i] = json.RawMessage(newRaw)
			}

			if !found {
				return fmt.Errorf("stage %q not found in pipeline %s", stageName, id)
			}

			body := map[string]any{"pipeline": map[string]any{"stages": updated}}
			res, err := apiclient.PatchJSONBodyJSONAPIResponse[PipelineAttrs](ctx, activeCtx, "/api/pipelines/"+id, body)
			if err != nil {
				return err
			}

			p := res.Attributes
			rows := [][]string{{res.ID, p.Name, p.Kind}}
			return r.Render(pipelineCols(), rows, res)
		},
	}

	cmd.Flags().StringVar(&role, "role", "", "Stage role (planning, implementing, reviewing, etc.)")
	cmd.Flags().StringVar(&stageType, "stage-type", "", "stage_type for custom stage types")
	cmd.Flags().StringVar(&customStageName, "custom-stage-name", "", "custom_stage_name (custom type only)")
	cmd.Flags().StringVar(&template, "template", "", "Template (custom type only)")
	cmd.Flags().StringVar(&instructions, "instructions", "", "Instructions / prompt text")
	cmd.Flags().StringVar(&gate, "gate", "", "Advance gate (e.g. manual, ci_pass)")
	cmd.Flags().StringVar(&agent, "agent", "", "Agent slug override")
	cmd.Flags().StringVar(&advanceNotice, "advance-notice", "", "Advance notice value")
	cmd.Flags().IntVar(&position, "position", 0, "Explicit position override")
	cmd.Flags().BoolVar(&runsInContainer, "runs-in-container", false, "runs_in_container override")
	cmd.Flags().StringVar(&onFailureJSON, "on-failure", "", `JSON: {"max_rework_count":3,"rework_to_position":0}`)
	cmd.Flags().StringVar(&promptSectionsJSON, "prompt-sections", "", "JSON array of prompt section objects")
	cmd.Flags().StringVar(&stageTriggersJSON, "stage-triggers", "", "JSON array of trigger strings")
	cmd.Flags().StringVar(&advanceReqJSON, "advance-requirements", "", "JSON array of requirement strings")
	cmd.Flags().StringVar(&branchCondJSON, "branch-conditions", "", "JSON array of branch condition strings")
	cmd.Flags().StringVar(&environmentJSON, "environment", "", "JSON object of environment key/value pairs")
	return cmd
}

func stagesRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <pipeline-id> <stage-name>",
		Short: "Remove a stage from a pipeline",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id := args[0]
			stageName := args[1]

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would PATCH /api/pipelines/%s to remove stage %q\n", id, stageName)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			// Read current stages.
			pipeline, err := apiclient.GetJSONAPISingle[PipelineAttrs](ctx, activeCtx, "/api/pipelines/"+id)
			if err != nil {
				return err
			}

			// Filter out the named stage.
			found := false
			filtered := make([]json.RawMessage, 0, len(pipeline.Attributes.Stages))
			for _, raw := range pipeline.Attributes.Stages {
				var stageMap map[string]any
				if err := json.Unmarshal(raw, &stageMap); err != nil {
					return fmt.Errorf("decoding stage: %w", err)
				}
				nameVal, _ := stageMap["name"].(string)
				if nameVal == stageName {
					found = true
					continue
				}
				filtered = append(filtered, raw)
			}

			if !found {
				return fmt.Errorf("stage %q not found in pipeline %s", stageName, id)
			}

			// If filtered is nil (last stage removed), use empty slice so JSON encodes as [].
			if filtered == nil {
				filtered = []json.RawMessage{}
			}

			body := map[string]any{"pipeline": map[string]any{"stages": filtered}}
			res, err := apiclient.PatchJSONBodyJSONAPIResponse[PipelineAttrs](ctx, activeCtx, "/api/pipelines/"+id, body)
			if err != nil {
				return err
			}

			p := res.Attributes
			rows := [][]string{{res.ID, p.Name, p.Kind}}
			return r.Render(pipelineCols(), rows, res)
		},
	}
}

// validateJSON parses raw JSON and returns it as json.RawMessage, or an error.
func validateJSON(raw, flag string) (json.RawMessage, error) {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil, fmt.Errorf("%s: invalid JSON: %w", flag, err)
	}
	return json.RawMessage(raw), nil
}
