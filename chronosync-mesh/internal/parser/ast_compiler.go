package parser

import (
	"fmt"
	"chronosync-mesh/config"
)

type ASTCompiler struct {
	routingRules map[string]config.SchemaRoutingRule
}

func NewASTCompiler(cfg *config.Config) *ASTCompiler {
	rules := make(map[string]config.SchemaRoutingRule)
	for _, rule := range cfg.SchemaRouting {
		rules[rule.Entity] = rule
	}
	return &ASTCompiler{routingRules: rules}
}

// Decompose processes the incoming unified blueprint into target database execution plans
func (ast *ASTCompiler) Decompose(req *UnifiedRequest) (*RelationalSubPlan, *GraphSubPlan, error) {
	// 1. Verify the entity exists in our declarative configuration mapping
	rule, exists := ast.routingRules[req.TargetEntity]
	if !exists {
		return nil, nil, fmt.Errorf("entity mapping context not found for: %s", req.TargetEntity)
	}

	// 2. Extract primary identification filter coordinate
	var primaryKeyVal any
	if len(rule.JoinKeys) > 0 {
		key := rule.JoinKeys[0]
		if val, ok := req.Filters[key]; ok {
			primaryKeyVal = val
		}
	}
	if primaryKeyVal == nil {
		return nil, nil, fmt.Errorf("missing mandatory primary lookup key filter attribute")
	}

	// 3. Compile the Relational Sub-Plan (SQL String Output)
	sqlString := fmt.Sprintf("SELECT id, username, email FROM user_profiles WHERE %s = ?;", rule.JoinKeys[0])
	relationalPlan := &RelationalSubPlan{
		QueryString: sqlString,
		FilterValue: primaryKeyVal,
	}

	// 4. Compile the Graph Sub-Plan (Cypher String Output)
	depthVal := req.IncludeRelations.Depth
	if depthVal <= 0 {
		depthVal = 1 // Safe fallback default
	}

	cypherString := fmt.Sprintf(
		"MATCH (u:User {user_id: $id})-[:%s*1..%d]->(f) RETURN f.user_id AS id, f.name AS name;",
		req.IncludeRelations.Type,
		depthVal,
	)
	
	graphPlan := &GraphSubPlan{
		QueryString: cypherString,
		FilterValue: primaryKeyVal,
		Depth:       depthVal,
	}

	return relationalPlan, graphPlan, nil
}
