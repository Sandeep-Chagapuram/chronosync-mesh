package parser

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

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
	if req.Operation != "READ_UNIFIED" {
		return nil, nil, fmt.Errorf("unsupported operation: %s", req.Operation)
	}
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
	primaryKey, err := normalizeID(primaryKeyVal)
	if err != nil {
		return nil, nil, err
	}

	// 3. Compile the Relational Sub-Plan (SQL String Output)
	sqlString := fmt.Sprintf("SELECT id, username, email FROM user_profiles WHERE %s = ?;", rule.JoinKeys[0])
	relationalPlan := &RelationalSubPlan{
		QueryString: sqlString,
		FilterValue: primaryKey,
	}

	// 4. Compile the Graph Sub-Plan (Cypher String Output)
	depthVal := req.IncludeRelations.Depth
	if depthVal <= 0 {
		depthVal = 1 // Safe fallback default
	}
	if depthVal > 5 {
		return nil, nil, fmt.Errorf("relation depth cannot exceed 5")
	}
	relationshipType := req.IncludeRelations.Type
	if relationshipType == "" {
		return nil, nil, fmt.Errorf("missing relationship type")
	}
	if !contains(rule.RelationshipTypes, relationshipType) {
		return nil, nil, fmt.Errorf("relationship type %q is not allowed for %s", relationshipType, req.TargetEntity)
	}

	cypherString := fmt.Sprintf(
		"MATCH (u:User {user_id: $id})-[:%s*1..%d]->(f) RETURN f.user_id AS id, f.name AS name;",
		relationshipType,
		depthVal,
	)

	graphPlan := &GraphSubPlan{
		QueryString: cypherString,
		FilterValue: primaryKey,
		Depth:       depthVal,
	}

	return relationalPlan, graphPlan, nil
}

// normalizeID makes JSON numeric identifiers and write-endpoint string identifiers
// address the same Neo4j property value.
func normalizeID(value any) (string, error) {
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return "", fmt.Errorf("primary lookup key cannot be empty")
		}
		return v, nil
	case float64:
		if v != float64(int64(v)) || v < 0 {
			return "", fmt.Errorf("primary lookup key must be a non-negative integer")
		}
		return strconv.FormatInt(int64(v), 10), nil
	case json.Number:
		n, err := v.Int64()
		if err != nil || n < 0 {
			return "", fmt.Errorf("primary lookup key must be a non-negative integer")
		}
		return strconv.FormatInt(n, 10), nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(v), nil
	default:
		return "", fmt.Errorf("primary lookup key has unsupported type %T", value)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
