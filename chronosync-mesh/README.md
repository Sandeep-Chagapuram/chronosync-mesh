# ChronoSync-Mesh

ChronoSync-Mesh is a Go HTTP proxy that accepts JSON requests and coordinates reads across MySQL and Neo4j. Client applications call the proxy over HTTP; they do not need either database driver.

## Run the complete stack with Docker

Install Docker Desktop and start its engine, then run from this directory:

    docker compose up --build

The proxy listens on http://localhost:8080. MySQL and Neo4j are available on ports 3306 and 7687. The first MySQL startup creates the user_profiles table. Database volumes persist between runs.

Send a unified read request:

    Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/v1/query -ContentType 'application/json' -Body '{"operation":"READ_UNIFIED","session_id":"demo","target_entity":"user_profile","filters":{"id":101},"include_relations":{"relation_name":"user_social_network","type":"FRIEND_OF","depth":2}}'

The sample write endpoint accepts entity_id, username, email, friend_id, and friend_name:

    Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/v1/write -ContentType 'application/json' -Body '{"entity_id":"101","username":"alex_tech","email":"alex@example.org","friend_id":204,"friend_name":"sarah_dev"}'

Stop the stack with docker compose down. To also erase persisted database data, run docker compose down -v.

## Neo4j does not become healthy

If Compose reports `container chronosync_neo4j is unhealthy`, first inspect its startup logs:

    docker compose logs neo4j

Neo4j persists its data and initial password in the `neo4j_data` volume. If this is a development installation with no data to retain, remove the old volumes and start again:

    docker compose down -v
    docker compose up --build

If the logs report an out-of-memory error, increase Docker Desktop's memory allocation and retry. The current health check allows up to ten and a half minutes for Neo4j to finish starting.

## Connect another backend

From a backend running on the same host, POST the unified JSON request to http://localhost:8080/api/v1/query and return the proxy JSON response to the frontend. A backend running in another Compose service should use http://proxy:8080/api/v1/query. Only the proxy container needs MySQL and Neo4j drivers.

## Local development

With Go 1.21 or newer installed, run from this directory:

    go run ./cmd/proxy

The supplied configuration targets Docker Compose service names (mysql and neo4j); change those addresses to localhost when running the proxy directly on the host.

## Configuration and demo credentials

Edit config/config.yaml and docker-compose.yml together when changing database credentials. The checked-in credentials are for local development only; replace them before exposing this stack beyond a trusted machine.
