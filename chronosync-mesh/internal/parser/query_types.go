package parser

// UnifiedRequest mirrors the incoming JSON blueprint from client apps
type UnifiedRequest struct {
	Operation        string           `json:"operation"`
	SessionID        string           `json:"session_id"`
	TargetEntity     string           `json:"target_entity"`
	Filters          map[string]any   `json:"filters"`
	IncludeRelations RelationSelector `json:"include_relations"`
}

type RelationSelector struct {
	RelationName string `json:"relation_name"`
	Type         string `json:"type"`
	Depth        int    `json:"depth"`
}

// SubPlans hold the individual decomposed query parameters
type RelationalSubPlan struct {
	QueryString string
	FilterValue any
}

type GraphSubPlan struct {
	QueryString string
	FilterValue any
	Depth       int
}
