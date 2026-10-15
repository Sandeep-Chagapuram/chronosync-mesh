package consistency

import (
	"chronosync-mesh/config"
	"chronosync-mesh/pkg/neo4j_driver"
	"context"
	"fmt"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"time"
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

		// Wait without holding database resources, and honor request cancellation.
		timer := time.NewTimer(retryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	return fmt.Errorf("graph replication did not reach token %d for entity %s within %s", expectedToken, entityID, maxStall)
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
			token, ok := val.(int64)
			if !ok || token < 0 {
				return 0, fmt.Errorf("invalid graph synchronization token %T", val)
			}
			return uint64(token), nil
		}
	}

	return 0, nil // Return baseline version if node doesn't exist yet
}
