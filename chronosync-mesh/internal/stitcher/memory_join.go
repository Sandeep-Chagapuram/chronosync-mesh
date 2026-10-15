package stitcher

import (
	"chronosync-mesh/internal/parser"
	"chronosync-mesh/pkg/mysql_driver"
	"chronosync-mesh/pkg/neo4j_driver"
	"context"
	"fmt"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"sync"
)

type StitchingEngine struct {
	mysql *mysql_driver.MySQLClient
	graph *neo4j_driver.Neo4jClient
}

type FinalOutput struct {
	ID        int            `json:"id"`
	Username  string         `json:"username"`
	Email     string         `json:"email"`
	Relations []FriendRecord `json:"user_social_network"`
}

type FriendRecord struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func NewStitchingEngine(mysql *mysql_driver.MySQLClient, graph *neo4j_driver.Neo4jClient) *StitchingEngine {
	return &StitchingEngine{mysql: mysql, graph: graph}
}

// ExecuteAndStitch orchestrates parallel data layer fetches and merges records with zero allocation bloat
func (se *StitchingEngine) ExecuteAndStitch(ctx context.Context, sqlPlan *parser.RelationalSubPlan, cypherPlan *parser.GraphSubPlan) (*FinalOutput, error) {
	var wg sync.WaitGroup
	var sqlErr, graphErr error

	output := &FinalOutput{
		Relations: make([]FriendRecord, 0),
	}

	wg.Add(2)

	// Thread Branch A: Concurrent Relational Fetch (MySQL)
	go func() {
		defer wg.Done()
		err := se.mysql.DB.QueryRowContext(ctx, sqlPlan.QueryString, sqlPlan.FilterValue).Scan(&output.ID, &output.Username, &output.Email)
		if err != nil {
			sqlErr = err
		}
	}()

	// Thread Branch B: Concurrent Graph Traversal (Neo4j)
	go func() {
		defer wg.Done()
		session := se.graph.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
		defer session.Close(ctx)

		res, err := session.Run(ctx, cypherPlan.QueryString, map[string]any{"id": cypherPlan.FilterValue})
		if err != nil {
			graphErr = err
			return
		}

		for res.Next(ctx) {
			rec := res.Record()
			var fid int64
			var fname string

			if val, ok := rec.Get("id"); ok && val != nil {
				value, ok := val.(int64)
				if !ok {
					graphErr = fmt.Errorf("neo4j returned non-integer friend id %T", val)
					return
				}
				fid = value
			}
			if val, ok := rec.Get("name"); ok && val != nil {
				value, ok := val.(string)
				if !ok {
					graphErr = fmt.Errorf("neo4j returned non-string friend name %T", val)
					return
				}
				fname = value
			}

			output.Relations = append(output.Relations, FriendRecord{
				ID:   int(fid),
				Name: fname,
			})
		}
		if err := res.Err(); err != nil {
			graphErr = err
		}
	}()

	// Wait for both execution pipelines to drop into the buffer memory structure
	wg.Wait()

	if sqlErr != nil {
		return nil, fmt.Errorf("mysql execution branch dropped: %w", sqlErr)
	}
	if graphErr != nil {
		return nil, fmt.Errorf("neo4j traversal branch dropped: %w", graphErr)
	}

	return output, nil
}
