package main

import (
	"strings"

	"github.com/AstraProtocol/astra-indexing/bootstrap/config"
)

type CustomConfig struct {
	ServerGithubAPI ServerGithubAPIConfig `yaml:"server_github_api"`
}

type ServerGithubAPIConfig struct {
	MigrationRepoRef string `yaml:"migration_repo_ref"`
}

type CLIConfig struct {
	LoggerColor *bool
	LogLevel    string

	DatabaseSSL      *bool
	DatabaseHost     string
	DatabasePort     *int32
	DatabaseUsername string
	DatabasePassword string
	DatabaseName     string
	DatabaseSchema   string

	TendermintHTTPRPCUrl string
	CosmosHTTPRPCUrl     string
	BlockscoutHTTPRPCUrl string
	JsonHTTPRPCUrl       string

	SwapLPAddress string

	GithubAPIUsername string
	GithubAPIToken    string

	CorsAllowedOrigins string

	IndexService *bool

	CronjobStats *bool

	CronjobReportDashboard *bool
	TikiAddress            string

	EnableConsumer          *bool
	ConsumerGroupId         string
	KafkaBrokers            string
	KafkaUser               string
	KafkaPassword           string
	KafkaAuthenticationType string
}

func OverrideByCLIConfig(config *config.Config, cliConfig *CLIConfig) {
	if cliConfig.LogLevel != "" {
		config.Logger.Level = cliConfig.LogLevel
	}
	if cliConfig.LoggerColor != nil {
		config.Logger.Color = *cliConfig.LoggerColor
	}
	if cliConfig.DatabaseSSL != nil {
		config.Postgres.SSL = *cliConfig.DatabaseSSL
	}
	if cliConfig.DatabaseHost != "" {
		config.Postgres.Host = cliConfig.DatabaseHost
	}
	if cliConfig.DatabasePort != nil {
		config.Postgres.Port = *cliConfig.DatabasePort
	}
	if cliConfig.DatabaseUsername != "" {
		config.Postgres.Username = cliConfig.DatabaseUsername
	}
	// Always overwrite database password with CLI (ENV)
	config.Postgres.Password = cliConfig.DatabasePassword
	if cliConfig.DatabaseName != "" {
		config.Postgres.Name = cliConfig.DatabaseName
	}
	if cliConfig.DatabaseSchema != "" {
		config.Postgres.Schema = cliConfig.DatabaseSchema
	}
	if cliConfig.TendermintHTTPRPCUrl != "" {
		config.TendermintApp.HTTPRPCUrl = cliConfig.TendermintHTTPRPCUrl
	}
	if cliConfig.CosmosHTTPRPCUrl != "" {
		config.CosmosApp.HTTPRPCUrl = cliConfig.CosmosHTTPRPCUrl
	}
	if cliConfig.BlockscoutHTTPRPCUrl != "" {
		config.BlockscoutApp.HTTPRPCUrl = cliConfig.BlockscoutHTTPRPCUrl
	}
	if cliConfig.JsonHTTPRPCUrl != "" {
		config.JsonrpcApp.HTTPJSONRPCUrl = cliConfig.JsonHTTPRPCUrl
	}
	if cliConfig.GithubAPIUsername != "" {
		config.IndexService.GithubAPI.Username = cliConfig.GithubAPIUsername
	}
	if cliConfig.GithubAPIToken != "" {
		config.IndexService.GithubAPI.Token = cliConfig.GithubAPIToken
	}
	if cliConfig.CorsAllowedOrigins != "" {
		config.HTTPService.CorsAllowedOrigins = []string{cliConfig.CorsAllowedOrigins}
	}
	if cliConfig.IndexService != nil {
		config.IndexService.Enable = *cliConfig.IndexService
	}
	if cliConfig.CronjobStats != nil {
		config.CronjobStats.Enable = *cliConfig.CronjobStats
	}
	if cliConfig.CronjobReportDashboard != nil {
		config.CronjobReportDashboard.Enable = *cliConfig.CronjobReportDashboard
	}
	if cliConfig.TikiAddress != "" {
		config.CronjobReportDashboard.TikiAddress = cliConfig.TikiAddress
	}
	if cliConfig.EnableConsumer != nil {
		config.KafkaService.EnableConsumer = *cliConfig.EnableConsumer
	}
	if cliConfig.ConsumerGroupId != "" {
		config.KafkaService.GroupID = cliConfig.ConsumerGroupId
	}
	if cliConfig.KafkaBrokers != "" {
		config.KafkaService.Brokers = strings.Split(strings.ReplaceAll(cliConfig.KafkaBrokers, " ", ""), ",")
	}
	if cliConfig.KafkaUser != "" {
		config.KafkaService.User = cliConfig.KafkaUser
	}
	if cliConfig.KafkaPassword != "" {
		config.KafkaService.Password = cliConfig.KafkaPassword
	}
	if cliConfig.KafkaAuthenticationType != "" {
		config.KafkaService.AuthenticationType = cliConfig.KafkaAuthenticationType
	}
	if cliConfig.SwapLPAddress != "" {
		config.SwapLPAddress = cliConfig.SwapLPAddress
	}
}
