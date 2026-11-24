package pipelines

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/httpclient"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
	"github.com/dotdevlabs/loopctl/internal/flagmeta"
)

// PipelineStageAttrs captures published workflow stage attributes.
// Name is *string because built-in stages return name: null.
type PipelineStageAttrs struct {
	StageType               string          `json:"stage_type"`
	CustomStageName         *string         `json:"custom_stage_name"`
	Gate                    string          `json:"gate"`
	ApprovalButtonText      *string         `json:"approval_button_text"`
	OnFailure               string          `json:"on_failure"`
	MaxReworkCount          *int            `json:"max_rework_count"`
	ReworkToPosition        *int            `json:"rework_to_position"`
	ReworkAllowedPositions  []int           `json:"rework_allowed_positions"`
	Position                int             `json:"position"`
	AgentID                 *string         `json:"agent_id"`
	EnvironmentID           *string         `json:"environment_id"`
	AdvanceRequirements     []string        `json:"advance_requirements"`
	FailureConditions       json.RawMessage `json:"failure_conditions,omitempty"`
	AdvanceNotice           *string         `json:"advance_notice"`
	RunsInContainerOverride *bool           `json:"runs_in_container_override"`
	PromptSections          json.RawMessage `json:"prompt_sections"`
	PromptConfiguration     json.RawMessage `json:"prompt_configuration"`
	StageTriggers           json.RawMessage `json:"stage_triggers"`
	Name                    *string         `json:"name"`
	Role                    string          `json:"role"`
	Instructions            string          `json:"instructions"`
}

// StageInput holds all pipeline stage attributes per the published API contract.
// Fields use omitempty to avoid inserting unset attributes or client defaults.
type StageInput struct {
	Name                    string          `json:"name,omitempty"`
	Role                    string          `json:"role,omitempty"`
	StageType               string          `json:"stage_type,omitempty"`
	CustomStageName         string          `json:"custom_stage_name,omitempty"`
	Instructions            string          `json:"instructions,omitempty"`
	Gate                    string          `json:"gate,omitempty"`
	ApprovalButtonText      *string         `json:"approval_button_text,omitempty"`
	AdvanceNotice           string          `json:"advance_notice,omitempty"`
	OnFailure               string          `json:"on_failure,omitempty"`
	MaxReworkCount          *int            `json:"max_rework_count,omitempty"`
	ReworkToPosition        *int            `json:"rework_to_position,omitempty"`
	ReworkAllowedPositions  *[]int          `json:"rework_allowed_positions,omitempty"`
	RunsInContainerOverride *bool           `json:"runs_in_container_override,omitempty"`
	Position                *int            `json:"position,omitempty"`
	AgentID                 *int64          `json:"agent_id,omitempty"`
	EnvironmentID           *int64          `json:"environment_id,omitempty"`
	PromptSections          json.RawMessage `json:"prompt_sections,omitempty"`
	PromptConfiguration     json.RawMessage `json:"prompt_configuration,omitempty"`
	StageTriggers           json.RawMessage `json:"stage_triggers,omitempty"`
	AdvanceRequirements     json.RawMessage `json:"advance_requirements,omitempty"`
}

func stageRow(id string, a PipelineStageAttrs) []string {
	name, agent, approval := "", "", ""
	if a.Name != nil {
		name = *a.Name
	}
	if a.AgentID != nil {
		agent = *a.AgentID
	}
	if a.ApprovalButtonText != nil {
		approval = *a.ApprovalButtonText
	}
	return []string{id, fmt.Sprint(a.Position), name, a.Role, a.StageType, a.Gate, agent, approval}
}

func stageCols() []output.Column {
	return []output.Column{
		{Header: "ID"},
		{Header: "POSITION"},
		{Header: "NAME"},
		{Header: "ROLE"},
		{Header: "STAGE_TYPE"},
		{Header: "GATE"},
		{Header: "AGENT"},
		{Header: "APPROVAL_BUTTON_TEXT"},
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

			res, err := apiclient.GetJSONAPISingleFull[PipelineAttrs](ctx, activeCtx, "/api/pipelines/"+id)
			if err != nil {
				return err
			}

			stages, err := relatedStages(res)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(stages))
			for _, stage := range stages {
				rows = append(rows, stageRow(stage.ID, stage.Attributes))
			}
			return r.Render(stageCols(), rows, stages)
		},
	}
}

const approvalButtonTextHelp = "Text for the human approval button; clear with --approval-button-text="

func stagesAddCmd() *cobra.Command {
	var (
		name                   string
		role                   string
		stageType              string
		customStageName        string
		instructions           string
		gate                   string
		approvalButtonText     string
		advanceNotice          string
		position               int
		onFailure              string
		maxReworkCount         int
		reworkToPosition       int
		reworkAllowedPositions []string
		runsInContainer        bool
		promptSectionsJSON     string
		stageTriggersJSON      string
		advanceReqJSON         string
	)

	cmd := &cobra.Command{
		Use:   "add <pipeline-id>",
		Short: "Add a stage to a pipeline",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id := args[0]

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
			if cmd.Flags().Changed("instructions") {
				stage.Instructions = instructions
			}
			if cmd.Flags().Changed("gate") {
				stage.Gate = gate
			}
			if cmd.Flags().Changed("approval-button-text") {
				stage.ApprovalButtonText = &approvalButtonText
			}
			if cmd.Flags().Changed("advance-notice") {
				stage.AdvanceNotice = advanceNotice
			}
			if cmd.Flags().Changed("position") {
				p := position
				stage.Position = &p
			}
			if cmd.Flags().Changed("on-failure") {
				stage.OnFailure = onFailure
			}
			if cmd.Flags().Changed("max-rework-count") {
				v := maxReworkCount
				stage.MaxReworkCount = &v
			}
			if cmd.Flags().Changed("rework-to-position") {
				v := reworkToPosition
				stage.ReworkToPosition = &v
			}
			if cmd.Flags().Changed("rework-allowed-positions") {
				positions, err := parseReworkPositions(reworkAllowedPositions)
				if err != nil {
					return err
				}
				stage.ReworkAllowedPositions = &positions
			}
			if cmd.Flags().Changed("runs-in-container") {
				b := runsInContainer
				stage.RunsInContainerOverride = &b
			}

			// Validate and set JSON fields.
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

			stageRaw, err := json.Marshal(stage)
			if err != nil {
				return fmt.Errorf("marshaling stage: %w", err)
			}
			path := "/api/pipelines/" + id
			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				fragment := map[string]any{"pipeline": map[string]any{"stages": []json.RawMessage{stageRaw}}}
				return previewRequest(cmd, "PATCH "+path+" (stage append fragment)", fragment)
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)
			pipeline, err := apiclient.GetJSONAPISingleFull[PipelineAttrs](ctx, activeCtx, path)
			if err != nil {
				return err
			}
			stages, err := replacementStages(pipeline, "")
			if err != nil {
				return err
			}
			stages = append(stages, stage)

			body := map[string]any{"pipeline": map[string]any{"stages": stages}}
			res, err := apiclient.PatchJSONBodyJSONAPIResponse[PipelineAttrs](ctx, activeCtx, path, body)
			if err != nil {
				return err
			}

			p := res.Attributes
			rows := [][]string{{res.ID, p.Name, p.Kind}}
			return r.Render(pipelineCols(), rows, res)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Legacy stage name; use --custom-stage-name")
	cmd.Flags().StringVar(&role, "role", "", "Legacy stage role")
	_ = cmd.Flags().MarkDeprecated("role", "Deprecated: use stage_type: 'custom'.")
	cmd.Flags().StringVar(&stageType, "stage-type", "", "Server stage type name")
	cmd.Flags().StringVar(&customStageName, "custom-stage-name", "", "custom_stage_name (custom type only)")
	cmd.Flags().StringVar(&instructions, "instructions", "", "Instructions / prompt text (deprecated: use --prompt-sections)")
	cmd.Flags().StringVar(&gate, "gate", "", "Advance gate (automated or human)")
	cmd.Flags().StringVar(&approvalButtonText, "approval-button-text", "", approvalButtonTextHelp)
	flagmeta.EnumFromSpec(cmd, "gate", "#/components/schemas/PipelineStageInput/properties/gate")
	cmd.Flags().StringVar(&advanceNotice, "advance-notice", "", "Advance notice value")
	cmd.Flags().IntVar(&position, "position", 0, "Explicit position override")
	cmd.Flags().StringVar(&onFailure, "on-failure", "", "Failure mode: rework or block")
	flagmeta.EnumFromSpec(cmd, "on-failure", "#/components/schemas/PipelineStageInput/properties/on_failure")
	cmd.Flags().IntVar(&maxReworkCount, "max-rework-count", 0, "Maximum number of rework iterations")
	cmd.Flags().IntVar(&reworkToPosition, "rework-to-position", 0, "Stage position to rework to")
	cmd.Flags().StringSliceVar(&reworkAllowedPositions, "rework-allowed-positions", nil, "Earlier stage positions allowed for rework")
	cmd.Flags().BoolVar(&runsInContainer, "runs-in-container", false, "runs_in_container_override flag")
	cmd.Flags().StringVar(&promptSectionsJSON, "prompt-sections", "", "JSON object of prompt section overrides")
	flagmeta.SchemaFromSpec(cmd, "prompt-sections", "#/components/schemas/PromptSections", false)
	cmd.Flags().StringVar(&stageTriggersJSON, "stage-triggers", "", "JSON array of stage trigger objects")
	flagmeta.SchemaFromSpec(cmd, "stage-triggers", "#/components/schemas/PipelineStageTrigger", true)
	cmd.Flags().StringVar(&advanceReqJSON, "advance-requirements", "", "JSON array of requirement strings")
	flagmeta.SchemaFromSpec(cmd, "advance-requirements", "#/components/schemas/PipelineStageInput/properties/advance_requirements", false)
	return cmd
}

func stagesUpdateCmd() *cobra.Command {
	var (
		role                   string
		stageType              string
		customStageName        string
		instructions           string
		gate                   string
		approvalButtonText     string
		advanceNotice          string
		position               int
		onFailure              string
		maxReworkCount         int
		reworkToPosition       int
		reworkAllowedPositions []string
		runsInContainer        bool
		promptSectionsJSON     string
		stageTriggersJSON      string
		advanceReqJSON         string
	)

	cmd := &cobra.Command{
		Use:   "update <pipeline-id> <stage-id>",
		Short: "Update a pipeline stage by id",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			pipelineID := args[0]
			stageID := args[1]

			// Validate at least one flag changed before any HTTP call.
			flagNames := []string{
				"role", "stage-type", "custom-stage-name", "instructions",
				"gate", "approval-button-text", "advance-notice", "position", "on-failure",
				"max-rework-count", "rework-to-position", "rework-allowed-positions", "runs-in-container",
				"prompt-sections", "stage-triggers", "advance-requirements",
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
			for flagName, val := range map[string]string{
				"prompt-sections":      promptSectionsJSON,
				"stage-triggers":       stageTriggersJSON,
				"advance-requirements": advanceReqJSON,
			} {
				if cmd.Flags().Changed(flagName) {
					if _, err := validateJSON(val, "--"+flagName); err != nil {
						return err
					}
				}
			}

			// Build patch body with only changed flags.
			patch := map[string]any{}
			if cmd.Flags().Changed("role") {
				patch["role"] = role
			}
			if cmd.Flags().Changed("stage-type") {
				patch["stage_type"] = stageType
			}
			if cmd.Flags().Changed("custom-stage-name") {
				patch["custom_stage_name"] = customStageName
			}
			if cmd.Flags().Changed("instructions") {
				patch["instructions"] = instructions
			}
			if cmd.Flags().Changed("gate") {
				patch["gate"] = gate
			}
			if cmd.Flags().Changed("approval-button-text") {
				patch["approval_button_text"] = approvalButtonText
			}
			if cmd.Flags().Changed("advance-notice") {
				patch["advance_notice"] = advanceNotice
			}
			if cmd.Flags().Changed("position") {
				patch["position"] = position
			}
			if cmd.Flags().Changed("on-failure") {
				patch["on_failure"] = onFailure
			}
			if cmd.Flags().Changed("max-rework-count") {
				patch["max_rework_count"] = maxReworkCount
			}
			if cmd.Flags().Changed("rework-to-position") {
				patch["rework_to_position"] = reworkToPosition
			}
			if cmd.Flags().Changed("rework-allowed-positions") {
				positions, err := parseReworkPositions(reworkAllowedPositions)
				if err != nil {
					return err
				}
				patch["rework_allowed_positions"] = positions
			}
			if cmd.Flags().Changed("runs-in-container") {
				patch["runs_in_container_override"] = runsInContainer
			}
			if cmd.Flags().Changed("prompt-sections") {
				var v any
				_ = json.Unmarshal([]byte(promptSectionsJSON), &v)
				patch["prompt_sections"] = v
			}
			if cmd.Flags().Changed("stage-triggers") {
				var v any
				_ = json.Unmarshal([]byte(stageTriggersJSON), &v)
				patch["stage_triggers"] = v
			}
			if cmd.Flags().Changed("advance-requirements") {
				var v any
				_ = json.Unmarshal([]byte(advanceReqJSON), &v)
				patch["advance_requirements"] = v
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			body := map[string]any{"pipeline_stage": patch}
			path := "/api/pipelines/" + pipelineID + "/stages/" + stageID
			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				return previewRequest(cmd, "PATCH "+path, body)
			}
			res, err := apiclient.PatchJSONBodyJSONAPIResponse[PipelineStageAttrs](ctx, activeCtx, path, body)
			if err != nil {
				return err
			}

			rows := [][]string{stageRow(res.ID, res.Attributes)}
			return r.Render(stageCols(), rows, res)
		},
	}

	cmd.Flags().StringVar(&role, "role", "", "Legacy stage role")
	_ = cmd.Flags().MarkDeprecated("role", "Deprecated: use stage_type: 'custom'.")
	cmd.Flags().StringVar(&stageType, "stage-type", "", "Server stage type name")
	cmd.Flags().StringVar(&customStageName, "custom-stage-name", "", "custom_stage_name (custom type only)")
	cmd.Flags().StringVar(&instructions, "instructions", "", "Instructions / prompt text (deprecated: use --prompt-sections)")
	cmd.Flags().StringVar(&gate, "gate", "", "Advance gate (automated or human)")
	cmd.Flags().StringVar(&approvalButtonText, "approval-button-text", "", approvalButtonTextHelp)
	flagmeta.EnumFromSpec(cmd, "gate", "#/components/schemas/PipelineStageInput/properties/gate")
	cmd.Flags().StringVar(&advanceNotice, "advance-notice", "", "Advance notice value")
	cmd.Flags().IntVar(&position, "position", 0, "Explicit position override")
	cmd.Flags().StringVar(&onFailure, "on-failure", "", "Failure mode: rework or block")
	flagmeta.EnumFromSpec(cmd, "on-failure", "#/components/schemas/PipelineStageInput/properties/on_failure")
	cmd.Flags().IntVar(&maxReworkCount, "max-rework-count", 0, "Maximum number of rework iterations")
	cmd.Flags().IntVar(&reworkToPosition, "rework-to-position", 0, "Stage position to rework to")
	cmd.Flags().StringSliceVar(&reworkAllowedPositions, "rework-allowed-positions", nil, "Earlier stage positions allowed for rework")
	cmd.Flags().BoolVar(&runsInContainer, "runs-in-container", false, "runs_in_container_override flag")
	cmd.Flags().StringVar(&promptSectionsJSON, "prompt-sections", "", "JSON object of prompt section overrides")
	flagmeta.SchemaFromSpec(cmd, "prompt-sections", "#/components/schemas/PromptSections", false)
	cmd.Flags().StringVar(&stageTriggersJSON, "stage-triggers", "", "JSON array of stage trigger objects")
	flagmeta.SchemaFromSpec(cmd, "stage-triggers", "#/components/schemas/PipelineStageTrigger", true)
	cmd.Flags().StringVar(&advanceReqJSON, "advance-requirements", "", "JSON array of requirement strings")
	flagmeta.SchemaFromSpec(cmd, "advance-requirements", "#/components/schemas/PipelineStageInput/properties/advance_requirements", false)
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
			pipeline, err := apiclient.GetJSONAPISingleFull[PipelineAttrs](ctx, activeCtx, "/api/pipelines/"+id)
			if err != nil {
				return err
			}

			filtered, err := replacementStages(pipeline, stageName)
			if err != nil {
				return err
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

// relatedStages follows only the pipeline's explicit stage linkage.
func relatedStages(res apiclient.SingleResult[PipelineAttrs]) ([]httpclient.Resource[PipelineStageAttrs], error) {
	var relationship struct {
		Data []apiclient.ResourceIdentifier `json:"data"`
	}
	if err := res.DecodeRelationship("stages", &relationship); err != nil {
		return nil, fmt.Errorf("decoding stages relationship: %w", err)
	}
	if relationship.Data == nil {
		return nil, fmt.Errorf("stages relationship data must be an array")
	}
	ids := relationship.Data
	included := make(map[apiclient.ResourceIdentifier]json.RawMessage)
	for _, resource := range res.IncludedResources() {
		included[apiclient.ResourceIdentifier{ID: resource.ID, Type: resource.Type}] = resource.Attributes
	}
	stages := make([]httpclient.Resource[PipelineStageAttrs], 0, len(ids))
	seen := make(map[apiclient.ResourceIdentifier]bool)
	for _, id := range ids {
		if id.ID == "" || id.Type != "pipeline_stages" {
			return nil, fmt.Errorf("invalid stages linkage: type %q id %q", id.Type, id.ID)
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		raw, ok := included[id]
		if !ok {
			return nil, fmt.Errorf("stage %s missing from included resources", id.ID)
		}
		var attrs PipelineStageAttrs
		if len(bytes.TrimSpace(raw)) == 0 || bytes.TrimSpace(raw)[0] != '{' {
			return nil, fmt.Errorf("stage %s attributes must be an object", id.ID)
		}
		if err := res.Decode("/included/*/attributes", raw, &attrs); err != nil {
			return nil, fmt.Errorf("decoding stage %s attributes: %w", id.ID, err)
		}
		stages = append(stages, httpclient.Resource[PipelineStageAttrs]{ID: id.ID, Type: id.Type, Attributes: attrs})
	}
	sort.SliceStable(stages, func(i, j int) bool { return stages[i].Attributes.Position < stages[j].Attributes.Position })
	return stages, nil
}

func replacementStages(pipeline apiclient.SingleResult[PipelineAttrs], remove string) ([]StageInput, error) {
	resources, err := relatedStages(pipeline)
	if err != nil {
		return nil, err
	}
	found := false
	stages := make([]StageInput, 0, len(resources))
	for _, resource := range resources {
		attrs := resource.Attributes
		name := dereference(attrs.Name)
		customName := dereference(attrs.CustomStageName)
		if remove != "" && (customName == remove || name == remove) {
			found = true
			continue
		}
		agentID, err := numericID(attrs.AgentID)
		if err != nil {
			return nil, fmt.Errorf("stage %s agent ID: %w", resource.ID, err)
		}
		environmentID, err := numericID(attrs.EnvironmentID)
		if err != nil {
			return nil, fmt.Errorf("stage %s environment ID: %w", resource.ID, err)
		}
		requirements, err := json.Marshal(attrs.AdvanceRequirements)
		if err != nil {
			return nil, err
		}
		stage := StageInput{
			StageType: attrs.StageType, CustomStageName: customName, Name: name, Instructions: attrs.Instructions,
			Gate: attrs.Gate, ApprovalButtonText: attrs.ApprovalButtonText, OnFailure: attrs.OnFailure,
			MaxReworkCount: attrs.MaxReworkCount, ReworkToPosition: attrs.ReworkToPosition,
			Position: &attrs.Position, RunsInContainerOverride: attrs.RunsInContainerOverride,
			AgentID: agentID, EnvironmentID: environmentID, AdvanceNotice: dereference(attrs.AdvanceNotice),
			PromptSections: attrs.PromptSections, StageTriggers: attrs.StageTriggers,
		}
		configuration := bytes.TrimSpace(attrs.PromptConfiguration)
		if len(configuration) > 0 && !bytes.Equal(configuration, []byte("{}")) {
			stage.PromptConfiguration = configuration
		}
		if attrs.AdvanceRequirements != nil {
			stage.AdvanceRequirements = requirements
		}
		if attrs.ReworkAllowedPositions != nil {
			stage.ReworkAllowedPositions = &attrs.ReworkAllowedPositions
		}
		stages = append(stages, stage)
	}
	if remove != "" && !found {
		return nil, fmt.Errorf("stage %q not found in pipeline %s", remove, pipeline.ID)
	}
	return stages, nil
}

func dereference(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func numericID(value *string) (*int64, error) {
	if value == nil {
		return nil, nil
	}
	id, err := strconv.ParseInt(*value, 10, 64)
	return &id, err
}

func parseReworkPositions(values []string) ([]int, error) {
	positions := make([]int, len(values))
	for i, value := range values {
		position, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("--rework-allowed-positions: invalid position %q", value)
		}
		positions[i] = position
	}
	return positions, nil
}
