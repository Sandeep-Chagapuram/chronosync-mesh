package config

import (
	"gopkg.in/yaml.v3"
	"os"
)

type Config struct {
	Server            ServerConfig        `yaml:"server"`
	Databases         DatabaseConfig      `yaml:"databases"`
	ConsistencyEngine ConsistencyConfig   `yaml:"consistency_engine"`
	SchemaRouting     []SchemaRoutingRule `yaml:"schema_routing_rules"`
}

type ServerConfig struct {
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Workers int    `yaml:"workers"`
}

type DatabaseConfig struct {
	Relational DBDetails `yaml:"relational_core"`
	Graph      DBDetails `yaml:"graph_core"`
}

type DBDetails struct {
	Engine   string `yaml:"engine"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"database_name"`
	URI      string `yaml:"uri"`
	PoolSize int    `yaml:"pool_size"`
}

type ConsistencyConfig struct {
	Mode          string           `yaml:"mode"`
	TokenTracking bool             `yaml:"token_tracking"`
	MaxSyncLag    int              `yaml:"max_sync_lag_tolerance_ms"`
	MicroStall    MicroStallConfig `yaml:"micro_stall"`
}

type MicroStallConfig struct {
	Enabled       bool `yaml:"enabled"`
	RetryInterval int  `yaml:"retry_interval_ms"`
	MaxTimeout    int  `yaml:"max_stall_timeout_ms"`
}

type SchemaRoutingRule struct {
	Entity            string   `yaml:"entity"`
	PrimaryStore      string   `yaml:"primary_store"`
	RelationshipTypes []string `yaml:"relationship_types,omitempty"`
	JoinKeys          []string `yaml:"join_keys"`
}

func LoadConfig(filename string) (*Config, error) {
	buf, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var c Config
	err = yaml.Unmarshal(buf, &c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
