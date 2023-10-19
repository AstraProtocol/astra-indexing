package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AstraProtocol/astra-indexing/cmd/astra-indexing/routes"
	"github.com/urfave/cli/v2"

	"github.com/AstraProtocol/astra-indexing/bootstrap"
	configuration "github.com/AstraProtocol/astra-indexing/bootstrap/config"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/AstraProtocol/astra-indexing/external/primptr"
	"github.com/AstraProtocol/astra-indexing/infrastructure"
	"github.com/AstraProtocol/astra-indexing/internal/evm"
	"github.com/AstraProtocol/astra-indexing/internal/filereader/yaml"
)

func run(args []string) error {
	cliApp := &cli.App{
		Name:                 filepath.Base(args[0]),
		Usage:                "Astra Chain Indexing Service",
		Version:              "v0.0.1",
		Copyright:            "(c) 2020-present Crypto.com",
		EnableBashCompletion: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Value: "./config/config.yaml",
				Usage: "YAML `FILE` to load configuration from",
			},

			&cli.StringFlag{
				Name:  "logLevel,l",
				Usage: "Log level (Allowed values: fatal,error,info,debug)",
			},
			&cli.BoolFlag{
				Name:  "color",
				Usage: "Display colored log",
			},

			&cli.BoolFlag{
				Name:    "dbSSL",
				Usage:   "Enable Postgres SSL mode",
				EnvVars: []string{"DB_SSL"},
			},
			&cli.StringFlag{
				Name:    "dbHost",
				Usage:   "Postgres database hostname",
				EnvVars: []string{"DB_HOST"},
			},
			&cli.UintFlag{
				Name:    "dbPort",
				Usage:   "Postgres database port",
				EnvVars: []string{"DB_PORT"},
			},
			&cli.StringFlag{
				Name:    "dbUsername",
				Usage:   "Postgres username",
				EnvVars: []string{"DB_USERNAME"},
			},
			&cli.StringFlag{
				Name:     "dbPassword",
				Usage:    "Postgres password",
				EnvVars:  []string{"DB_PASSWORD"},
				Required: true,
			},
			&cli.StringFlag{
				Name:    "dbName",
				Usage:   "Postgres database name",
				EnvVars: []string{"DB_NAME"},
			},
			&cli.StringFlag{
				Name:    "dbSchema",
				Usage:   "Postgres schema name",
				EnvVars: []string{"DB_SCHEMA"},
			},

			&cli.StringFlag{
				Name:    "tendermintURL",
				Usage:   "Tendermint HTTP RPC URL",
				EnvVars: []string{"TENDERMINT_URL"},
			},
			&cli.StringFlag{
				Name:    "cosmosAppURL",
				Usage:   " Cosmos App RPC URL",
				EnvVars: []string{"COSMOSAPP_URL"},
			},
			&cli.StringFlag{
				Name:    "blockscoutURL",
				Usage:   "Blockscout HTTP RPC URL",
				EnvVars: []string{"BLOCKSCOUT_URL"},
			},
			&cli.StringFlag{
				Name:    "JsonRpcURL",
				Usage:   "Json HTTP RPC URL",
				EnvVars: []string{"JSONRPC_URL"},
			},
			&cli.StringFlag{
				Name:    "corsAllowedOrigins",
				Usage:   "Cors Allowed Origins",
				EnvVars: []string{"CORS_ALLOWED_ORIGINS"},
			},
			&cli.BoolFlag{
				Name:    "indexService",
				Usage:   "Enable Index Service",
				EnvVars: []string{"INDEX_SERVICE"},
			},
			&cli.BoolFlag{
				Name:    "cronjobStats",
				Usage:   "Enable Cronjob Statistics",
				EnvVars: []string{"STATISTICS_SERVICE"},
			},
			&cli.BoolFlag{
				Name:    "cronjobReportDashboard",
				Usage:   "Enable Cronjob Report Dashboard",
				EnvVars: []string{"REPORT_DASHBOARD_SERVICE"},
			},
			&cli.StringFlag{
				Name:    "tikiAddress",
				Usage:   "Tiki Pool Address",
				EnvVars: []string{"TIKI_ADDRESS"},
			},
			&cli.StringFlag{
				Name:    "startingBlockHeight",
				Usage:   "Starting Block Height",
				EnvVars: []string{"STARTING_BLOCK_HEIGHT"},
			},
			&cli.BoolFlag{
				Name:    "enableConsumer",
				Usage:   "Enable Consumer",
				EnvVars: []string{"ENABLE_CONSUMER"},
			},
			&cli.StringFlag{
				Name:    "consumerGroupId",
				Usage:   "Kafka Consumer Group Id",
				EnvVars: []string{"CONSUMER_GROUP_ID"},
			},
			&cli.StringFlag{
				Name:    "kafkaBrokers",
				Usage:   "Kafka Brokers List",
				EnvVars: []string{"KAFKA_BROKERS"},
			},
			&cli.StringFlag{
				Name:    "kafkaUser",
				Usage:   "Kafka Username",
				EnvVars: []string{"KAFKA_USER"},
			},
			&cli.StringFlag{
				Name:    "kafkaPassword",
				Usage:   "Kafka Password",
				EnvVars: []string{"KAFKA_PASSWORD"},
			},
			&cli.StringFlag{
				Name:    "kafkaAuthenticationType",
				Usage:   "Kafka Authentication Type",
				EnvVars: []string{"KAFKA_AUTHEN_TYPE"},
			},
			&cli.StringFlag{
				Name:    "caCertPath",
				Usage:   "Ca Cert Path",
				EnvVars: []string{"KAFKA_CA_CERT_PATH"},
			},
			&cli.StringFlag{
				Name:    "tlsCertPath",
				Usage:   "Tls Cert Path",
				EnvVars: []string{"KAFKA_TLS_CERT_PATH"},
			},
			&cli.StringFlag{
				Name:    "tlsKeyPath",
				Usage:   "Tls Key Path",
				EnvVars: []string{"KAFKA_TLS_KEY_PATH"},
			},
		},
		Action: func(ctx *cli.Context) error {
			if args := ctx.Args(); args.Len() > 0 {
				return fmt.Errorf("unexpected arguments: %q", args.Get(0))
			}

			// Prepare FileConfig
			configPath := ctx.String("config")
			var config configuration.Config
			err := yaml.FromYAMLFile(configPath, &config)
			if err != nil {
				return fmt.Errorf("error config from yaml: %v", err)
			}

			var customConfig CustomConfig
			err = yaml.FromYAMLFile(configPath, &customConfig)
			if err != nil {
				return fmt.Errorf("error custom config from yaml: %v", err)
			}

			cliConfig := CLIConfig{
				LogLevel: ctx.String("logLevel"),

				DatabaseHost:     ctx.String("dbHost"),
				DatabaseUsername: ctx.String("dbUsername"),
				DatabasePassword: ctx.String("dbPassword"),
				DatabaseName:     ctx.String("dbName"),
				DatabaseSchema:   ctx.String("dbSchema"),

				TendermintHTTPRPCUrl: ctx.String("tendermintURL"),
				CosmosHTTPRPCUrl:     ctx.String("cosmosAppURL"),
				BlockscoutHTTPRPCUrl: ctx.String("blockscoutURL"),
				JsonHTTPRPCUrl:       ctx.String("JsonRpcURL"),

				GithubAPIUsername: ctx.String("githubAPIUsername"),
				GithubAPIToken:    ctx.String("githubAPIToken"),

				CorsAllowedOrigins: ctx.String("corsAllowedOrigins"),
			}
			if ctx.IsSet("color") {
				cliConfig.LoggerColor = primptr.Bool(ctx.Bool("color"))
			}
			if ctx.IsSet("dbSSL") {
				cliConfig.DatabaseSSL = primptr.Bool(ctx.Bool("dbSSL"))
			}
			if ctx.IsSet("dbPort") {
				cliConfig.DatabasePort = primptr.Int32(int32(ctx.Int("dbPort")))
			}
			if ctx.IsSet("indexService") {
				cliConfig.IndexService = primptr.Bool(ctx.Bool("indexService"))
			}
			if ctx.IsSet("cronjobStats") {
				cliConfig.CronjobStats = primptr.Bool(ctx.Bool("cronjobStats"))
			}
			if ctx.IsSet("cronjobReportDashboard") {
				cliConfig.CronjobReportDashboard = primptr.Bool(ctx.Bool("cronjobReportDashboard"))
			}
			if ctx.IsSet("tikiAddress") {
				cliConfig.TikiAddress = ctx.String("tikiAddress")
			}
			if ctx.IsSet("startingBlockHeight") {
				cliConfig.StartingBlockHeight = primptr.Int64(int64(ctx.Int("startingBlockHeight")))
			}
			if ctx.IsSet("enableConsumer") {
				cliConfig.EnableConsumer = primptr.Bool(ctx.Bool("enableConsumer"))
			}
			if ctx.IsSet("consumerGroupId") {
				cliConfig.ConsumerGroupId = ctx.String("consumerGroupId")
			}
			if ctx.IsSet("kafkaBrokers") {
				cliConfig.KafkaBrokers = ctx.String("kafkaBrokers")
			}
			if ctx.IsSet("kafkaUser") {
				cliConfig.KafkaUser = ctx.String("kafkaUser")
			}
			if ctx.IsSet("kafkaPassword") {
				cliConfig.KafkaPassword = ctx.String("kafkaPassword")
			}
			if ctx.IsSet("kafkaAuthenticationType") {
				cliConfig.KafkaAuthenticationType = ctx.String("kafkaAuthenticationType")
			}
			if ctx.IsSet("caCertPath") {
				cliConfig.CaCertPath = ctx.String("caCertPath")
			}
			if ctx.IsSet("tlsCertPath") {
				cliConfig.TlsCertPath = ctx.String("tlsCertPath")
			}
			if ctx.IsSet("tlsKeyPath") {
				cliConfig.TlsKeyPath = ctx.String("tlsKeyPath")
			}

			OverrideByCLIConfig(&config, &cliConfig)

			// Create logger
			logLevel := parseLogLevel(config.Logger.Level)
			logger := infrastructure.NewZerologLogger(os.Stdout)
			logger.SetLogLevel(logLevel)

			evmUtil, err := evm.NewEvmUtils()
			if err != nil {
				return err
			}

			app := bootstrap.NewApp(logger, &config, evmUtil)

			app.InitIndexService(
				initProjections(logger, app.GetRDbConn(), &config, &customConfig, evmUtil),
				nil,
			)
			app.InitHTTPAPIServer(routes.InitRouteRegistry(logger, app.GetRDbConn(), &config, evmUtil))

			app.RunCronJobsStats(app.GetRDbConn().ToHandle())

			app.RunCronJobsReportDashboard(app.GetRDbConn().ToHandle())

			app.Run()

			return nil
		},
	}

	err := cliApp.Run(args)
	if err != nil {
		return err
	}

	return nil
}

func parseLogLevel(level string) applogger.LogLevel {
	switch level {
	case "panic":
		return applogger.LOG_LEVEL_PANIC
	case "error":
		return applogger.LOG_LEVEL_ERROR
	case "info":
		return applogger.LOG_LEVEL_INFO
	case "debug":
		return applogger.LOG_LEVEL_DEBUG
	default:
		return applogger.LOG_LEVEL_INFO
	}
}
