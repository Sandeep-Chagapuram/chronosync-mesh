
ChronoSync-Mesh: Complete System Specification & Implementation Prompt


System Core Objectives

You are an expert distributed systems engineer and database architect. Your task is to build ChronoSync-Mesh: a high-performance, language-agnostic, out-of-band multi-model database proxy middleware written in Go (Golang) (or alternative high-performance backend stack). This middleware acts as a reverse proxy sitting transparently between any backend client application and a hybrid database cluster consisting of a relational database (MySQL/PostgreSQL) and a graph database (Neo4j).
The application must be completely decoupled, language-agnostic, and plug-and-play. It must communicate exclusively over highly optimized HTTP REST endpoints using standardized structured JSON payloads. This architecture isolates application backends from the downstream data layout, transforming complex multi-database orchestration into a simple configuration-driven networking layer.

Real-World Gaps Fixed:

1. The Analytics Cross-Model Join Blindspot: Eliminates application-level looping over massive database queries. It parses a single hybrid query, executes sub-plans in parallel, and merges datasets efficiently in memory using structural zero-copy buffer layouts.
2. Replication Sync Drift & Lag: Eliminates the infrastructure complexity of Change Data Capture (CDC) pipelines (e.g., Debezium, Kafka) by utilizing lightweight internal token tracking directly within the middleware data flight path.
3. Read-Your-Own-Writes (RYOW) Session Anomalies: Introduces a non-blocking Token-Driven Micro-Stall protocol that pauses subsequent read requests for a fraction of a millisecond if the secondary graph engine has not fully synchronized with the relational core, preventing stale reads without distributed locking.

Part 1: Structural File & Folder Directory Layout

Generate the core project folder structure precisely as detailed below. This structure maintains clean isolation of concerns across all modules:
text
chronosync-mesh/
├── config/
│   ├── config.yaml            # Central configuration file for routing and credentials
│   └── config_loader.go       # Parses config.yaml into system-wide struct definitions
├── internal/
│   ├── parser/
│   │   ├── ast_compiler.go    # Abstract Syntax Tree parser for query decomposition
│   │   └── query_types.go     # Structured JSON input models and internal sub-plan definitions
│   ├── router/
│   │   ├── cost_estimator.go  # Metric-based/heuristic routing cost engine
│   │   └── parallel_worker.go # Orchestrates concurrent execution to MySQL and Neo4j
│   ├── consistency/
│   │   ├── token_ledger.go    # Lightweight in-memory monotonic version clock tracker
│   │   └── micro_stall.go     # Non-blocking lock-free request stalling logic
│   └── stitcher/
│       └── memory_join.go     # Stream-aligned data stitching framework (Apache Arrow style)
├── pkg/
│   ├── mysql_driver/
│   │   └── client.go          # Connection pooling, transaction handles, and write interceptors
│   └── neo4j_driver/
│       └── client.go          # Connection pools and transaction execution hooks for Bolt paths
├── cmd/
│   └── proxy/
│       └── main.go            # Entrypoint. Binds proxy to network port 8080 and handles startup
├── go.mod                     # Project dependencies
└── README.md                  # System manual and deployment documentation
Use code with caution.

Part 2: Configuration Mapping Setup

Implement a configuration parsing model that reads the global declarative mappings from a central config file.

Required config/config.yaml:

yaml
server:
  host: "0.0.0.0"
  port: 8080
  workers: 8

databases:
  relational_core:
    engine: "mysql"
    host: "localhost"
    port: 3306
    user: "root"
    password: "secure_password_123"
    database_name: "chronosync_db"
    pool_size: 50

  graph_core:
    engine: "neo4j"
    uri: "bolt://localhost:7687"
    user: "neo4j"
    password: "graph_secure_password"
    pool_size: 50

consistency_engine:
  mode: "session-aware"
  token_tracking: true
  max_sync_lag_tolerance_ms: 15
  micro_stall:
    enabled: true
    retry_interval_ms: 2
    max_stall_timeout_ms: 12

schema_routing_rules:
  - entity: "user_profile"
    primary_store: "relational_core"
    join_keys: ["id"]
  - entity: "user_social_network"
    primary_store: "graph_core"
    relationship_types: ["FRIEND_OF", "FOLLOWS"]
    join_keys: ["user_id"]
Use code with caution.

Part 3: Step-by-Step Module Implementation Specifications


Module 1: The Unified Query Interface & AST Compiler (internal/parser/)

1. Expose a single endpoint: POST /api/v1/query.
2. Accept a structured unified JSON request payload mapping the requested properties precisely like this:json
{
  "operation": "READ_UNIFIED",
  "session_id": "sess_998124",
  "target_entity": "user_profile",
  "filters": { "id": 101 },
  "include_relations": {
    "relation_name": "user_social_network",
    "type": "FRIEND_OF",
    "depth": 2
  }
}
Use code with caution.
3. Create the ast_compiler.go component. It must dissect this JSON object at runtime.
4. Using the schema_routing_rules from config.yaml, split this request into two clear query instructions:
	• Relational Sub-Plan: An SQL statement string: SELECT id, username, email FROM user_profiles WHERE id = 101;
	• Graph Sub-Plan: A Cypher statement string: MATCH (u:User {user_id: 101})-[:FRIEND_OF*1..2]->(f) RETURN f.user_id AS id, f.name AS name;

Module 2: The Monotonic Token Ledger & Micro-Stall Guardrail (internal/consistency/)

1. Implement a lock-free internal hash map inside the middleware tracking Entity_ID -> Latest_Write_Token.
2. When a WRITE payload passes through the middleware (e.g., creating a new relationship node):
	• First execute the write on the primary relational engine.
	• Intercept the successful transactional write, issue a monotonic logical token (e.g., an incremented scalar integer counter or atomic nanosecond vector timestamp), and store it inside the middleware ledger.
	• Send the write update asynchronously to Neo4j.
3. When a subsequent READ_UNIFIED payload hits the proxy for that specific entity, intercept the query flow:
	• Fetch the expected token version for that target session_id or entity from your middleware memory map.
	• Query the current metadata state directly from the Neo4j instance to retrieve its latest synchronized transaction token.
	• The Micro-Stall Block: If Neo4j_Token < Relational_Token, do not execute the read traversal yet. Block the executing worker routing channel, sleep for retry_interval_ms (2ms), and recheck. Execute this non-blocking loop continuously until the graph engine caught up or max_stall_timeout_ms (12ms) expires. If timeout expires, execute fallback: route the request to fetch the connection dependencies safely out of the relational engine tables directly to avoid returning corrupt or stale graph representations.

Module 3: Zero-Copy Serialization & Execution Stitcher (internal/stitcher/)

1. Execute the Relational Sub-Plan and Graph Sub-Plan in parallel using independent Go concurrency routines (go routines via channels).
2. Collect the response arrays from both downstream database execution lines.
3. Instead of parsing structures into heavy application object maps using nested lookup loops (O(N²)), align rows by their mapped keys (join_keys: ["id"]) inside a continuous memory array buffer layout (mimicking columnar structures found in Apache Arrow).
4. Zip the datasets into a single unified JSON output layout programmatically before dispatching it out of the middleware socket:json
{
  "status": "success",
  "execution_mode": "CHRONOSYNC_MESH_PROXY",
  "consistency_verified_token": 4012,
  "data": {
    "id": 101,
    "username": "alex_tech",
    "email": "alex@tech.org",
    "user_social_network": [
      { "id": 204, "name": "sarah_dev", "depth": 1 },
      { "id": 309, "name": "john_ops", "depth": 2 }
    ]
  }
}
Use code with caution.

Part 4: Production Evaluation & Benchmark Harness Setup

To guarantee that this project generates verifiable performance numbers for academic paper publication, implement a dedicated tracking script or framework:
1. Provide a benchmarking test toggle inside the main system startup process to allow manual switching between performance configurations for isolated load testing.
2. Ensure the system captures and explicitly outputs metrics tracking:
	• End-to-End Execution Latency: Timestamps at the exact moment of request entry vs. absolute exit.
	• Throughput Performance: Total volume of complete client request lifecycles processed over a fixed timeframe.
	• Data Drift Counter: Logs any instances where data read mismatches are exposed to client views during intense concurrent operations.

Deployment & Usage Instructions for Later Integration

Write clean, error-resilient code utilizing native standard drivers (go-sql-driver/mysql and neo4j/neo4j-go-driver). Ensure the middleware exposes complete startup log metrics over the server socket terminal.
Any application backend built later (whether in Python, Node.js, or Java) can fully utilize this project by dropping its internal database drivers entirely, starting this proxy engine, and routing structural data requests purely through HTTP calls directly to http://localhost:8080/api/v1/query. Initialize generation of all functional project modules now.

