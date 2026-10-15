package neo4j_driver

import (
	"context"
	"://github.com"
	"chronosync-mesh/config"
)

type Neo4jClient struct {
	Driver neo4j.DriverWithContext
}

func NewNeo4jClient(cfg config.DBDetails) (*Neo4jClient, error) {
	driver, err := neo4j.NewDriverWithContext(
		cfg.URI,
		neo4j.BasicAuth(cfg.User, cfg.Password, ""),
	)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	err = driver.VerifyConnectivity(ctx)
	if err != nil {
		return nil, err
	}

	return &Neo4jClient{Driver: driver}, nil
}
