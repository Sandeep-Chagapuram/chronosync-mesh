package consistency

import (
	"context"
	"fmt"
	"time"
	"://github.com"
	"chronosync-mesh/config"
	"chronosync-mesh/pkg/neo4j_driver"
)

type MicroStallGuardrail struct {
	ledger *TokenLedger
	cfg    config.MicroStallConfig
	graph  *neo4j_driver.Neo4jClient
}

func NewMicroStallGuardrail(ledger *TokenLedger, cfg config.MicroStallConfig, graph *neo4j_driver.Neo4jClient) *MicroStallGuardrail {
	return &MicroStallGuardrail{
		ledger: ledger,
		cfg:    cfg,
		graph:  graph,
	}
}

// EnforceSessionSafety executes a timed check loop to absorb database synchronization drift
func (msg *MicroStallGuardrail) EnforceSessionSafety(ctx context.Context, entityID string) error {
	if !msg.cfg.Enabled {
		return nil
	}

	expectedToken := msg.ledger.GetLatestToken(entityID)
	if expectedToken == 0 {
		return nil // No writes exist, read can proceed instantly
	}

	startTime := time.Now()
	maxStall := time.Duration(msg.cfg.MaxTimeout) * time.Millisecond
	retryInterval := time.Duration(msg.cfg.RetryInterval) * time.Millisecond

	for {
		// Query the latest synchronized transaction metadata version token from the Graph Engine
		graphSyncToken, err := msg.fetchGraphMetadataToken(ctx, entityID)
		if err != nil {
			return fmt.Errorf("failed to poll graph metadata context: %w", err)
		}

		// If the Graph DB has caught up, break the stall execution block safely
		if graphSyncToken >= expectedToken {
			return nil
		}

		// Fallback check: If the maximum allowed stall timeout window has expired, fall back safely
		if time.Since(startTime) >= maxStall {
			break
		}

		// Non-blocking wait state to let replication threads catch up
		time.Sleep(retryInterval)
	}

	return nil
}

func (msg *MicroStallGuardrail) fetchGraphMetadataToken(ctx context.Context, entityID string) (uint64, error) {
	session := msg.graph.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	// Fetch version metadata node properties from our target graph structure
	query := "MATCH (m:SyncMetadata {entity_id: $entity_id}) RETURN m.last_updated_token AS token"
	result, err := session.Run(ctx, query, map[string]any{"entity_id": entityID})
	if err != nil {
		return 0, err
	}

	if result.Next(ctx) {
		record := result.Record()
		if val, ok := record.Get("token"); ok && val != nil {
			return uint64(val.(int64)), nil
		}
	}

	return 0, nil // Return baseline version if node doesn't exist yet
}
