package routes

import (
	"github.com/crypto-com/chain-indexing/appinterface/cosmosapp"
	"github.com/crypto-com/chain-indexing/appinterface/rdb"
	"github.com/crypto-com/chain-indexing/appinterface/tendermint"
	"github.com/crypto-com/chain-indexing/bootstrap"
	"github.com/crypto-com/chain-indexing/bootstrap/config"
	applogger "github.com/crypto-com/chain-indexing/external/logger"
	cosmosapp_infrastructure "github.com/crypto-com/chain-indexing/infrastructure/cosmosapp"
	httpapi_handlers "github.com/crypto-com/chain-indexing/infrastructure/httpapi/handlers"
	tendermint_infrastructure "github.com/crypto-com/chain-indexing/infrastructure/tendermint"
)

func InitRouteRegistry(
	logger applogger.Logger,
	rdbConn rdb.Conn,
	config *config.Config,
) bootstrap.RouteRegistry {
	var cosmosAppClient cosmosapp.Client

	cosmosAppClient = cosmosapp_infrastructure.NewHTTPClient(
		config.CosmosApp.HTTPRPCUrl,
		config.Blockchain.BondingDenom,
	)

	var tendermintClient tendermint.Client
	tendermintClient = tendermint_infrastructure.NewHTTPClient(
		config.TendermintApp.HTTPRPCUrl,
		config.TendermintApp.StrictGenesisParsing,
	)

	validatorAddressPrefix := config.Blockchain.ValidatorAddressPrefix
	conNodeAddressPrefix := config.Blockchain.ConNodeAddressPrefix

	routes := make([]Route, 0)
	searchHandler := httpapi_handlers.NewSearch(logger, rdbConn.ToHandle())
	routes = append(routes,
		Route{
			Method:  GET,
			path:    "api/v1/search",
			handler: searchHandler.Search,
		},
	)

	blocksHandler := httpapi_handlers.NewBlocks(logger, rdbConn.ToHandle())
	routes = append(routes,
		Route{
			Method:  GET,
			path:    "api/v1/blocks",
			handler: blocksHandler.List,
		},
		Route{
			Method:  GET,
			path:    "api/v1/blocks/{height-or-hash}",
			handler: blocksHandler.FindBy,
		},
		Route{
			Method:  GET,
			path:    "api/v1/blocks/{height}/transactions",
			handler: blocksHandler.ListTransactionsByHeight,
		},
	)

	accountsHandlers := httpapi_handlers.NewAccounts(
		logger,
		rdbConn.ToHandle(),
		cosmosAppClient,
		config.Blockchain.ValidatorAddressPrefix,
	)
	routes = append(routes,
		Route{
			Method:  GET,
			path:    "api/v1/accounts",
			handler: accountsHandlers.List,
		},
		Route{
			Method:  GET,
			path:    "api/v1/accounts/{account}",
			handler: accountsHandlers.FindBy,
		},
	)

	accountMessagesHandlers := httpapi_handlers.NewAccountMessages(logger, rdbConn.ToHandle())
	routes = append(routes,
		Route{
			Method:  GET,
			path:    "api/v1/accounts/{account}/messages",
			handler: accountMessagesHandlers.ListByAccount,
		},
	)

	accountTransactionsHandler := httpapi_handlers.NewAccountTransactions(logger, rdbConn.ToHandle())
	routes = append(routes,
		Route{
			Method:  GET,
			path:    "api/v1/accounts/{account}/transactions",
			handler: accountTransactionsHandler.ListByAccount,
		},
	)

	statusHandlers := httpapi_handlers.NewStatusHandler(logger, cosmosAppClient, rdbConn.ToHandle())
	routes = append(routes,
		Route{
			Method:  GET,
			path:    "api/v1/status",
			handler: statusHandlers.GetStatus,
		},
	)

	transactionHandler := httpapi_handlers.NewTransactions(logger, rdbConn.ToHandle())
	routes = append(routes,
		Route{
			Method:  GET,
			path:    "api/v1/transactions",
			handler: transactionHandler.List,
		},
		Route{
			Method:  GET,
			path:    "api/v1/transactions/{hash}",
			handler: transactionHandler.FindByHash,
		},
	)

	proposalsHandler := httpapi_handlers.NewProposals(
		logger,
		rdbConn.ToHandle(),
		cosmosAppClient,
	)
	routes = append(routes,
		Route{
			Method:  GET,
			path:    "api/v1/proposals",
			handler: proposalsHandler.List,
		},
		Route{
			Method:  GET,
			path:    "api/v1/proposals/{id}",
			handler: proposalsHandler.FindById,
		},
		Route{
			Method:  GET,
			path:    "api/v1/proposals/{id}/votes",
			handler: proposalsHandler.ListVotesById,
		},
		Route{
			Method:  GET,
			path:    "api/v1/proposals/{id}/depositors",
			handler: proposalsHandler.ListDepositorsById,
		},
	)

	validatorsHandler := httpapi_handlers.NewValidators(
		logger,
		validatorAddressPrefix,
		conNodeAddressPrefix,
		cosmosAppClient,
		tendermintClient,
		rdbConn.ToHandle(),
	)
	routes = append(routes,
		Route{
			Method:  GET,
			path:    "api/v1/validators",
			handler: validatorsHandler.List,
		},
		Route{
			Method:  GET,
			path:    "api/v1/validators/active",
			handler: validatorsHandler.ListActive,
		},
		Route{
			Method:  GET,
			path:    "api/v1/validators/{address}",
			handler: validatorsHandler.FindBy,
		},
		Route{
			Method:  GET,
			path:    "api/v1/validators/{address}/activities",
			handler: validatorsHandler.ListActivities,
		},
	)

	ibcChannelHandler := httpapi_handlers.NewIBCChannel(
		logger,
		rdbConn.ToHandle(),
	)
	routes = append(routes,
		Route{
			Method:  GET,
			path:    "api/v1/ibc/channels",
			handler: ibcChannelHandler.ListChannels,
		},
		Route{
			Method:  GET,
			path:    "api/v1/ibc/channels/{channelId}",
			handler: ibcChannelHandler.FindChannelById,
		},
		Route{
			Method:  GET,
			path:    "api/v1/ibc/denom-hash-mappings",
			handler: ibcChannelHandler.ListAllDenomHashMapping,
		},
	)

	ibcChannelMessageHandler := httpapi_handlers.NewIBCChannelMessage(
		logger,
		rdbConn.ToHandle(),
	)
	routes = append(routes,
		Route{
			Method:  GET,
			path:    "api/v1/ibc/channels/{channelId}/messages",
			handler: ibcChannelMessageHandler.ListByChannelID,
		},
	)

	return &RouteRegistry{routes: routes}
}
