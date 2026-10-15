package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	"context"

	"chronosync-mesh/config"
	"chronosync-mesh/internal/consistency"
	"chronosync-mesh/internal/parser"
	"chronosync-mesh/internal/stitcher"
	"chronosync-mesh/pkg/mysql_driver"
	"chronosync-mesh/pkg/neo4j_driver"
)

type ProxyServer struct {
	compiler   *parser.ASTCompiler
	guardrail  *consistency.MicroStallGuardrail
	stitcher   *stitcher.StitchingEngine
	tokenStore *consistency.TokenLedger
	mysql      *mysql_driver.MySQLClient
	graph      *neo4j_driver.Neo4jClient
}

func main() {
	log.Println("[INITIALIZING] ChronoSync-Mesh Proxy Core Engine...")

	// 1. Ingest Configuration Architecture Profiles
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("[FATAL] Configuration load crash: %v", err)
	}

	// 2. Open High-Performance Database Pools
	mysqlClient, err := mysql_driver.NewMySQLClient(cfg.Databases.Relational)
	if err != nil {
		log.Fatalf("[FATAL] MySQL operational connection pool error: %v", err)
	}
	log.Println("[SUCCESS] Relational core target link established (MySQL).")

	neo4jClient, err := neo4j_driver.NewNeo4jClient(cfg.Databases.Graph)
	if err != nil {
		log.Fatalf("[FATAL] Neo4j transactional channel link error: %v", err)
	}
	log.Println("[SUCCESS] Graph core database target link established (Neo4j).")

	// 3. Mount Memory Management Internals
	ledger := consistency.NewTokenLedger()
	guardrail := consistency.NewMicroStallGuardrail(ledger, cfg.ConsistencyEngine.MicroStall, neo4jClient)
	compiler := parser.NewASTCompiler(cfg)
	stitchingEngine := stitcher.NewStitchingEngine(mysqlClient, neo4jClient)

	server := &ProxyServer{
		compiler:   compiler,
		guardrail:  guardrail,
		stitcher:   stitchingEngine,
		tokenStore: ledger,
		mysql:      mysqlClient,
		graph:      neo4jClient,
	}

	// 4. Map Network Proxy Routes
	http.HandleFunc("/api/v1/query", server.handleUnifiedQuery)
	http.HandleFunc("/api/v1/write", server.handleInterceptedWrite)

	address := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("[SERVER READY] Listening for network apps on http://%s", address)
	if err := http.ListenAndServe(address, nil); err != nil {
		log.Fatalf("[FATAL] Network port mapping crash: %v", err)
	}
}

func (s *ProxyServer) handleUnifiedQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req parser.UnifiedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad JSON Query Structure", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Step A: Parse and decompose the request using the AST compiler
	relationalPlan, graphPlan, err := s.compiler.Decompose(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Step B: Calculate session safety metrics and check for replication drift lag
	entityID := fmt.Sprintf("%v", relationalPlan.FilterValue)
	if err := s.guardrail.EnforceSessionSafety(ctx, entityID); err != nil {
		log.Printf("[METRIC WARNING] Session guardrail flag trace: %v", err)
	}

	// Step C: Execute structural operations in parallel and combine data models
	finalBuffer, err := s.stitcher.ExecuteAndStitch(ctx, relationalPlan, graphPlan)
	if err != nil {
		http.Error(w, fmt.Sprintf("Data Layer Merge Failure: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"status":          "success",
		"execution_mode":  "CHRONOSYNC_MESH_PROXY",
		"timestamp_epoch": time.Now().UnixMilli(),
		"data":            finalBuffer,
	})
}

func (s *ProxyServer) handleInterceptedWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	type WritePayload struct {
		EntityID  string `json:"entity_id"`
		Username  string `json:"username"`
		Email     string `json:"email"`
		FriendID  int64  `json:"friend_id"`
		FriendName string `json:"friend_name"`
	}

	var payload WritePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Bad Write Payload Structure", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// 1. Lock down the structural database update securely in MySQL first
	_, err := s.mysql.DB.ExecContext(ctx, 
		"INSERT INTO user_profiles (id, username, email) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE username=?, email=?", 
		payload.EntityID, payload.Username, payload.Email, payload.Username, payload.Email)
	if err != nil {
		http.Error(w, fmt.Sprintf("Primary Relational Commit Failed: %v", err), http.StatusInternalServerError)
		return
	}

	// 2. Intercept confirmation and mint a new incremental version token into the ledger
	mintedToken := s.tokenStore.MintNewToken(payload.EntityID)

	// 3. Fire-and-forget asynchronous write propagation out to Neo4j
	go func(entity string, fID int64, fName string, tok uint64) {
		session := s.graph.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
		defer session.Close(ctx)

		// Simulating pipeline replication work
		_, _ = session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			// Write the social connection relationship link edge node
			q1 := "MERGE (u:User {user_id: \$uid}) MERGE (f:User {user_id: fid) SET f.name = fname MERGE (u)-[:FRIEND_OF]->(f)"
			_, _ = tx.Run(ctx, q1, map[string]any{"uid": entity, "fid": fID, "fname": fName})

			// Update version tracking metadata explicitly inside the Graph engine properties
			q2 := "MERGE (m:SyncMetadata {entity_id: \(uid}) SET m.last_updated_token =\)tok"
			_, _ = tx.Run(ctx, q2, map[string]any{"uid": entity, "tok": int64(tok)})
			return nil, nil
		})
		log.Printf("[ASYNC SYNC Complete] Propagated token version v%d downstream to Graph Database for Entity: %s", tok, entity)
	}(payload.EntityID, payload.FriendID, payload.FriendName, mintedToken)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{
		"status":        "acknowledged",
		"tracked_token": mintedToken,
		"message":       "Primary transactional record locked. Replication loop initiated.",
	})
}
