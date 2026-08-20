package schema

// OperationKey uniquely identifies an API operation.
type OperationKey struct {
	Method string // uppercase HTTP method
	Path   string // loopctl template path (":param" style)
}

// OperationCoverage declares how loopctl covers one spec operation.
type OperationCoverage struct {
	// Command is the loopctl command path, e.g. "tasks comments create".
	Command string
	// QueryParams lists the query parameters this command actually sends.
	// Must be a subset of what the spec declares for this operation.
	QueryParams []string
	// Paginated is true when the command follows links.next pagination.
	Paginated bool
}

// Covered maps every loopctl-covered operation to its coverage metadata.
// Adding a new loopctl command requires a new entry here.
var Covered = map[OperationKey]OperationCoverage{
	{Method: "POST", Path: "/api/registrations"}:                     {Command: "onboard"},
	{Method: "GET", Path: "/api/platforms"}:                          {Command: "platforms list", Paginated: true},
	{Method: "GET", Path: "/api/task_kinds"}:                         {Command: "task-kinds list", Paginated: true},
	{Method: "POST", Path: "/api/task_kinds"}:                        {Command: "task-kinds create"},
	{Method: "GET", Path: "/api/pipelines"}:                          {Command: "pipelines list", Paginated: true},
	{Method: "POST", Path: "/api/pipelines"}:                         {Command: "pipelines create"},
	{Method: "GET", Path: "/api/pipelines/:id"}:                      {Command: "pipelines get"},
	{Method: "PATCH", Path: "/api/pipelines/:id"}:                    {Command: "pipelines update"},
	{Method: "GET", Path: "/api/projects"}:                           {Command: "projects list", Paginated: true},
	{Method: "POST", Path: "/api/projects"}:                          {Command: "projects create"},
	{Method: "GET", Path: "/api/projects/:id"}:                       {Command: "projects get"},
	{Method: "PATCH", Path: "/api/projects/:id"}:                     {Command: "projects update"},
	{Method: "GET", Path: "/api/tasks"}:                              {Command: "tasks list", QueryParams: []string{"project_id"}, Paginated: true},
	{Method: "POST", Path: "/api/tasks"}:                             {Command: "tasks create"},
	{Method: "GET", Path: "/api/tasks/:id"}:                          {Command: "tasks get"},
	{Method: "PATCH", Path: "/api/tasks/:id"}:                        {Command: "tasks update"},
	{Method: "POST", Path: "/api/tasks/:task_id/cancellation"}:       {Command: "tasks cancel"},
	{Method: "POST", Path: "/api/tasks/:task_id/unblock"}:            {Command: "tasks unblock"},
	{Method: "GET", Path: "/api/tasks/:task_id/activities"}:          {Command: "tasks watch (internal poll)", Paginated: true},
	{Method: "POST", Path: "/api/tasks/:task_id/comments"}:           {Command: "tasks comments create"},
	{Method: "GET", Path: "/api/tasks/:task_id/todos"}:               {Command: "tasks todos list", QueryParams: []string{"stage_name"}, Paginated: true},
	{Method: "POST", Path: "/api/tasks/:task_id/todos"}:              {Command: "tasks todos create"},
	{Method: "PATCH", Path: "/api/tasks/:task_id/todos/:id"}:         {Command: "tasks todos update"},
	{Method: "GET", Path: "/api/account_pipeline_defaults"}:          {Command: "task-kinds list-default-pipelines", Paginated: true},
	{Method: "PATCH", Path: "/api/account_pipeline_defaults/:kind"}:  {Command: "task-kinds set-default-pipeline"},
	{Method: "DELETE", Path: "/api/account_pipeline_defaults/:kind"}: {Command: "task-kinds clear-default-pipeline"},
}

// Excluded maps operations intentionally not covered by loopctl with the reason.
// These are server-internal, UI-only, container-internal, or otherwise not agent-facing.
var Excluded = map[OperationKey]string{
	{Method: "GET", Path: "/status"}:                                                     "server health probe; not agent-facing",
	{Method: "GET", Path: "/api/agents"}:                                                 "admin read-only; loopctl acts as an agent, not an agent manager",
	{Method: "PATCH", Path: "/api/containers/:id"}:                                       "container-internal: agent container self-reports its own status",
	{Method: "POST", Path: "/api/pipelines/:pipeline_id/clone"}:                          "not yet implemented in loopctl",
	{Method: "GET", Path: "/api/tasks/:id/logs"}:                                         "Loki log streaming; not yet implemented in loopctl",
	{Method: "GET", Path: "/api/tasks/:task_id/recordings"}:                              "recording browser; not yet implemented in loopctl",
	{Method: "POST", Path: "/api/tasks/:task_id/recordings"}:                             "recording upload; not yet implemented in loopctl",
	{Method: "GET", Path: "/api/tasks/:task_id/recordings/:id"}:                          "recording browser; not yet implemented in loopctl",
	{Method: "GET", Path: "/api/tasks/:task_id/recordings/:recording_id/content"}:        "binary asciinema stream; not yet implemented in loopctl",
	{Method: "GET", Path: "/api/tasks/:task_id/images/:id"}:                              "binary image serve; web-UI-only",
	{Method: "POST", Path: "/api/tasks/:task_id/completion"}:                             "legacy endpoint superseded by stage_completion; kept in spec for backcompat",
	{Method: "POST", Path: "/api/tasks/:task_id/stage_completion"}:                       "called by loopcontrol agent infrastructure, not a direct loopctl user command",
	{Method: "POST", Path: "/api/tasks/:task_id/bootstrap_failure"}:                      "container-internal: agent container reports bootstrap failures",
	{Method: "POST", Path: "/api/tasks/:task_id/pull_request"}:                           "called by loopcontrol agent infrastructure, not a direct loopctl user command",
	{Method: "POST", Path: "/api/tasks/:task_id/prompt_sync"}:                            "container-internal: agent container syncs prompt hash",
	{Method: "GET", Path: "/api/tasks/:task_id/transitions"}:                             "audit history; not yet implemented in loopctl",
	{Method: "GET", Path: "/api/tasks/:task_id/containers"}:                              "ops monitoring; not yet implemented in loopctl",
	{Method: "POST", Path: "/api/tasks/:task_id/restart"}:                                "ops operation; not yet implemented in loopctl",
	{Method: "GET", Path: "/api/planning_sessions/:planning_session_id/drafts"}:          "planning-agent-internal; not a human/script operation",
	{Method: "POST", Path: "/api/planning_sessions/:planning_session_id/drafts"}:         "planning-agent-internal; not a human/script operation",
	{Method: "DELETE", Path: "/api/planning_sessions/:planning_session_id/drafts/:id"}:   "planning-agent-internal; not a human/script operation",
	{Method: "POST", Path: "/api/planning_sessions/:planning_session_id/completion"}:     "planning-agent-internal; not a human/script operation",
	{Method: "GET", Path: "/api/projects/:project_slug/flow_registry_flows"}:             "flow registry; not yet implemented in loopctl",
	{Method: "POST", Path: "/api/flow_registry_flows/:flow_registry_flow_id/recordings"}: "flow registry; not yet implemented in loopctl",
	{Method: "POST", Path: "/api/flow_registry_flows/:flow_registry_flow_id/runs"}:       "flow registry; not yet implemented in loopctl",
	{Method: "GET", Path: "/api/github/organizations"}:                                   "used by web UI repo picker; not loopctl-facing",
	{Method: "GET", Path: "/api/github/organizations/:org/repositories"}:                 "used by web UI repo picker; not loopctl-facing",
	{Method: "GET", Path: "/api/github/repositories"}:                                    "used by web UI repo picker; not loopctl-facing",
	{Method: "POST", Path: "/api/github/repositories"}:                                   "used by web UI repo creation flow; not loopctl-facing",
	{Method: "GET", Path: "/api/github/repositories/:owner/:repo/branches"}:              "used by web UI branch picker; not loopctl-facing",
	{Method: "GET", Path: "/api/billing/funding_envelope"}:                               "topup command uses /api/topup (legacy); billing/* endpoints not yet migrated in loopctl",
	{Method: "POST", Path: "/api/billing/x402_payments"}:                                 "topup command uses /api/topup (legacy); billing/* endpoints not yet migrated in loopctl",
	{Method: "POST", Path: "/api/billing/l402_payments"}:                                 "topup command uses /api/topup (legacy); billing/* endpoints not yet migrated in loopctl",
	{Method: "GET", Path: "/api/billing/topup_packages"}:                                 "not yet implemented in loopctl",
	{Method: "POST", Path: "/api/billing/topups"}:                                        "not yet implemented in loopctl",
	{Method: "POST", Path: "/api/billing/usdc_payments"}:                                 "internal crediting endpoint; not agent-facing",
	// PUT aliases for PATCH operations — loopctl uses PATCH exclusively.
	{Method: "PUT", Path: "/api/pipelines/:id"}:                   "PUT alias for PATCH /api/pipelines/:id; loopctl uses PATCH",
	{Method: "PUT", Path: "/api/projects/:id"}:                    "PUT alias for PATCH /api/projects/:id; loopctl uses PATCH",
	{Method: "PUT", Path: "/api/containers/:id"}:                  "PUT alias for PATCH /api/containers/:id; container-internal, loopctl uses PATCH",
	{Method: "PUT", Path: "/api/tasks/:id"}:                       "PUT alias for PATCH /api/tasks/:id; loopctl uses PATCH",
	{Method: "PUT", Path: "/api/tasks/:task_id/todos/:id"}:        "PUT alias for PATCH /api/tasks/:task_id/todos/:id; loopctl uses PATCH",
	{Method: "PUT", Path: "/api/account_pipeline_defaults/:kind"}: "PUT alias for PATCH /api/account_pipeline_defaults/:kind; loopctl uses PATCH",
}
