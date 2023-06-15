package main

import (
	"strings"

	evmUtil "github.com/AstraProtocol/astra-indexing/internal/evm"
	"github.com/AstraProtocol/astra-indexing/projection/blockevent"

	"github.com/AstraProtocol/astra-indexing/appinterface/cosmosapp"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	configuration "github.com/AstraProtocol/astra-indexing/bootstrap/config"
	projection_entity "github.com/AstraProtocol/astra-indexing/entity/projection"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	cosmosapp_infrastructure "github.com/AstraProtocol/astra-indexing/infrastructure/cosmosapp"
	"github.com/AstraProtocol/astra-indexing/infrastructure/pg"
	"github.com/AstraProtocol/astra-indexing/infrastructure/pg/migrationhelper"
	github_migrationhelper "github.com/AstraProtocol/astra-indexing/infrastructure/pg/migrationhelper/github"
	"github.com/AstraProtocol/astra-indexing/projection/account"
	"github.com/AstraProtocol/astra-indexing/projection/account_message"
	"github.com/AstraProtocol/astra-indexing/projection/account_transaction"
	"github.com/AstraProtocol/astra-indexing/projection/block"
	"github.com/AstraProtocol/astra-indexing/projection/chainstats"
	"github.com/AstraProtocol/astra-indexing/projection/ibc_channel"
	"github.com/AstraProtocol/astra-indexing/projection/ibc_channel_message"
	"github.com/AstraProtocol/astra-indexing/projection/proposal"
	"github.com/AstraProtocol/astra-indexing/projection/transaction"
	"github.com/AstraProtocol/astra-indexing/projection/validator"
	"github.com/AstraProtocol/astra-indexing/projection/validatorstats"
)

func initProjections(
	logger applogger.Logger,
	rdbConn rdb.Conn,
	config *configuration.Config,
	customConfig *CustomConfig,
	evmUtil evmUtil.EvmUtils,
) []projection_entity.Projection {
	if !config.IndexService.Enable {
		return []projection_entity.Projection{}
	}

	cosmosAppClient := cosmosapp_infrastructure.NewHTTPClient(
		config.CosmosApp.HTTPRPCUrl,
		config.Blockchain.BondingDenom,
	)

	projections := make([]projection_entity.Projection, 0, len(config.IndexService.Projection.Enables))
	initParams := InitProjectionParams{
		Logger:  logger,
		RdbConn: rdbConn,

		ExtraConfigs: config.IndexService.Projection.ExtraConfigs,

		CosmosAppClient:       cosmosAppClient,
		AccountAddressPrefix:  config.Blockchain.AccountAddressPrefix,
		ConsNodeAddressPrefix: config.Blockchain.ConNodeAddressPrefix,

		GithubAPIUser:    config.IndexService.GithubAPI.Username,
		GithubAPIToken:   config.IndexService.GithubAPI.Token,
		MigrationRepoRef: config.IndexService.GithubAPI.MigrationRepoRef,

		ServerMigrationRepoRef: customConfig.ServerGithubAPI.MigrationRepoRef,
	}

	for _, projectionName := range config.IndexService.Projection.Enables {
		projection := InitProjection(
			projectionName, initParams,
			evmUtil,
		)
		if projection == nil {
			continue
		} else {
			if onInitErr := projection.OnInit(); onInitErr != nil {
				logger.Errorf(
					"error initializing projection %s, system will attempt to initialize the projection again on next restart: %v",
					projection.Id(), onInitErr,
				)
				continue
			}
		}
		projections = append(projections, projection)
	}

	logger.Infof("Enabled the follow projection: [%s]", strings.Join(config.IndexService.Projection.Enables, ", "))

	return projections
}

func InitProjection(name string, params InitProjectionParams, util evmUtil.EvmUtils) projection_entity.Projection {
	connString := params.RdbConn.(*pg.PgxConn).ConnString()

	githubMigrationHelperConfig := github_migrationhelper.Config{
		GithubAPIUser:    params.GithubAPIUser,
		GithubAPIToken:   params.GithubAPIToken,
		MigrationRepoRef: params.MigrationRepoRef,
		ConnString:       connString,
	}
	sourceURL := github_migrationhelper.GenerateDefaultSourceURL(name, githubMigrationHelperConfig)
	databaseURL := migrationhelper.GenerateDefaultDatabaseURL(name, connString)
	migrationHelper := github_migrationhelper.NewGithubMigrationHelper(sourceURL, databaseURL)
	switch name {
	case "Account":
		if params.GithubAPIToken == "" {
			return account.NewAccount(params.Logger, params.RdbConn, params.CosmosAppClient, nil)
		}
		return account.NewAccount(params.Logger, params.RdbConn, params.CosmosAppClient, migrationHelper)
	case "AccountTransaction":
		if params.GithubAPIToken == "" {
			return account_transaction.NewAccountTransaction(params.Logger, params.RdbConn, params.AccountAddressPrefix, nil, util)
		}
		return account_transaction.NewAccountTransaction(params.Logger, params.RdbConn, params.AccountAddressPrefix, migrationHelper, util)
	case "AccountMessage":
		if params.GithubAPIToken == "" {
			return account_message.NewAccountMessage(params.Logger, params.RdbConn, params.AccountAddressPrefix, nil)
		}
		return account_message.NewAccountMessage(params.Logger, params.RdbConn, params.AccountAddressPrefix, migrationHelper)
	case "Block":
		if params.GithubAPIToken == "" {
			return block.NewBlock(params.Logger, params.RdbConn, nil)
		}
		return block.NewBlock(params.Logger, params.RdbConn, migrationHelper)
	case "BlockEvent":
		sourceURL := github_migrationhelper.GenerateSourceURL(
			github_migrationhelper.MIGRATION_GITHUB_URL_FORMAT,
			params.GithubAPIUser,
			params.GithubAPIToken,
			blockevent.MIGRATION_DIRECOTRY,
			params.MigrationRepoRef,
		)
		databaseURL := migrationhelper.GenerateDefaultDatabaseURL(name, connString)
		migrationHelper := github_migrationhelper.NewGithubMigrationHelper(sourceURL, databaseURL)
		if params.GithubAPIToken == "" {
			return blockevent.NewBlockEvent(params.Logger, params.RdbConn, nil)
		}
		return blockevent.NewBlockEvent(params.Logger, params.RdbConn, migrationHelper)
	case "ChainStats":
		sourceURL = github_migrationhelper.GenerateSourceURL(
			github_migrationhelper.MIGRATION_GITHUB_URL_FORMAT,
			params.GithubAPIUser,
			params.GithubAPIToken,
			chainstats.MIGRATION_DIRECOTRY,
			params.MigrationRepoRef,
		)
		databaseURL = migrationhelper.GenerateDefaultDatabaseURL(name, connString)
		migrationHelper = github_migrationhelper.NewGithubMigrationHelper(sourceURL, databaseURL)
		if params.GithubAPIToken == "" {
			return chainstats.NewChainStats(params.Logger, params.RdbConn, nil)
		}
		return chainstats.NewChainStats(params.Logger, params.RdbConn, migrationHelper)
	case "Proposal":
		if params.GithubAPIToken == "" {
			return proposal.NewProposal(params.Logger, params.RdbConn, params.ConsNodeAddressPrefix, params.CosmosAppClient, nil)
		}
		return proposal.NewProposal(params.Logger, params.RdbConn, params.ConsNodeAddressPrefix, params.CosmosAppClient, migrationHelper)
	case "Transaction":
		if params.GithubAPIToken == "" {
			return transaction.NewTransaction(params.Logger, params.RdbConn, nil, util)
		}
		return transaction.NewTransaction(params.Logger, params.RdbConn, migrationHelper, util)
	case "Validator":
		if params.GithubAPIToken == "" {
			return validator.NewValidator(params.Logger, params.RdbConn, params.ConsNodeAddressPrefix, nil)
		}
		return validator.NewValidator(params.Logger, params.RdbConn, params.ConsNodeAddressPrefix, migrationHelper)
	case "ValidatorStats":
		sourceURL = github_migrationhelper.GenerateSourceURL(
			github_migrationhelper.MIGRATION_GITHUB_URL_FORMAT,
			params.GithubAPIUser,
			params.GithubAPIToken,
			validatorstats.MIGRATION_DIRECOTRY,
			params.MigrationRepoRef,
		)
		databaseURL = migrationhelper.GenerateDatabaseURL(
			connString,
			validatorstats.MIGRATION_TABLE_NAME,
		)
		migrationHelper = github_migrationhelper.NewGithubMigrationHelper(sourceURL, databaseURL)
		if params.GithubAPIToken == "" {
			return validatorstats.NewValidatorStats(params.Logger, params.RdbConn, nil)
		}
		return validatorstats.NewValidatorStats(params.Logger, params.RdbConn, migrationHelper)
	case "IBCChannel":
		return ibc_channel.NewIBCChannel(
			params.Logger,
			params.RdbConn,
			&ibc_channel.Config{
				EnableTxMsgTrace: false,
			},
			migrationHelper,
		)
	case "IBCChannelTxMsgTrace":
		sourceURL = github_migrationhelper.GenerateSourceURL(
			github_migrationhelper.MIGRATION_GITHUB_URL_FORMAT,
			params.GithubAPIUser,
			params.GithubAPIToken,
			ibc_channel.MIGRATION_DIRECOTRY,
			params.MigrationRepoRef,
		)
		databaseURL = migrationhelper.GenerateDatabaseURL(
			connString,
			ibc_channel.MIGRATION_TABLE_NAME,
		)
		migrationHelper = github_migrationhelper.NewGithubMigrationHelper(sourceURL, databaseURL)
		if params.GithubAPIToken == "" {
			return ibc_channel.NewIBCChannel(
				params.Logger,
				params.RdbConn,
				&ibc_channel.Config{
					EnableTxMsgTrace: true,
				},
				nil,
			)
		}
		return ibc_channel.NewIBCChannel(
			params.Logger,
			params.RdbConn,
			&ibc_channel.Config{
				EnableTxMsgTrace: true,
			},
			migrationHelper,
		)
	case "IBCChannelMessage":
		if params.GithubAPIToken == "" {
			return ibc_channel_message.NewIBCChannelMessage(params.Logger, params.RdbConn, nil)
		}
		return ibc_channel_message.NewIBCChannelMessage(params.Logger, params.RdbConn, migrationHelper)
	}

	return nil
}

type InitProjectionParams struct {
	Logger  applogger.Logger
	RdbConn rdb.Conn

	ExtraConfigs map[string]interface{}

	CosmosAppClient       cosmosapp.Client
	AccountAddressPrefix  string
	ConsNodeAddressPrefix string

	GithubAPIUser    string
	GithubAPIToken   string
	MigrationRepoRef string

	ServerMigrationRepoRef string
}
