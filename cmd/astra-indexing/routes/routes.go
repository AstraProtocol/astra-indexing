package routes

import (
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/bootstrap"
	"github.com/AstraProtocol/astra-indexing/bootstrap/config"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	blockscout_infrastructure "github.com/AstraProtocol/astra-indexing/infrastructure/blockscout"
	cosmosapp_infrastructure "github.com/AstraProtocol/astra-indexing/infrastructure/cosmosapp"
	httpapi_handlers "github.com/AstraProtocol/astra-indexing/infrastructure/httpapi/handlers"
	tendermint_infrastructure "github.com/AstraProtocol/astra-indexing/infrastructure/tendermint"
)

func InitRouteRegistry(
	logger applogger.Logger,
	rdbConn rdb.Conn,
	config *config.Config,
) bootstrap.RouteRegistry {
	cosmosAppClient := cosmosapp_infrastructure.NewHTTPClient(
		config.CosmosApp.HTTPRPCUrl,
		config.Blockchain.BondingDenom,
	)

	tendermintClient := tendermint_infrastructure.NewHTTPClient(
		config.TendermintApp.HTTPRPCUrl,
		config.TendermintApp.StrictGenesisParsing,
	)

	blockscoutClient := blockscout_infrastructure.NewHTTPClient(
		logger,
		config.BlockscoutApp.HTTPRPCUrl,
	)

	validatorAddressPrefix := config.Blockchain.ValidatorAddressPrefix
	conNodeAddressPrefix := config.Blockchain.ConNodeAddressPrefix

	routes := make([]Route, 0)
	searchHandler := httpapi_handlers.NewSearch(logger, *blockscoutClient, rdbConn.ToHandle())
	routes = append(routes,
		Route{
			Method:  GET,
			path:    "api/v1/search",
			handler: searchHandler.Search,
		},
	)

	blocksHandler := httpapi_handlers.NewBlocks(logger, rdbConn.ToHandle(), *blockscoutClient)
	routes = append(routes,
		Route{
			Method:  GET,
			path:    "api/v1/blocks",
			handler: blocksHandler.List,
		},
		Route{
			Method:  GET,
			path:    "api/v1/eth-block-number",
			handler: blocksHandler.EthBlockNumber,
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
		*blockscoutClient,
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
		Route{
			Method:  GET,
			path:    "api/v1/accounts/get-list-tokens",
			handler: accountsHandlers.GetListTokens,
		},
		Route{
			Method:  GET,
			path:    "api/v1/accounts/detail/{account}",
			handler: accountsHandlers.GetDetailAddress,
		},
	)

	accountTransactionsHandler := httpapi_handlers.NewAccountTransactions(
		logger, rdbConn.ToHandle(),
		cosmosAppClient,
		*blockscoutClient,
	)
	routes = append(routes,
		Route{
			Method:  GET,
			path:    "api/v1/accounts/{account}/transactions",
			handler: accountTransactionsHandler.ListByAccount,
		},
		Route{
			Method:  GET,
			path:    "api/v1/accounts/{account}/counters",
			handler: accountTransactionsHandler.GetCounters,
		},
		Route{
			Method:  GET,
			path:    "api/v1/accounts/get-top-addresses-balance",
			handler: accountTransactionsHandler.GetTopAddressesBalance,
		},
	)

	statsHandlers := httpapi_handlers.NewStatsHandler(
		logger,
		*blockscoutClient,
		rdbConn.ToHandle(),
	)
	routes = append(routes,
		Route{
			Method:  GET,
			path:    "api/v1/estimate-counted-info",
			handler: statsHandlers.EstimateCounted,
		},
		Route{
			Method:  GET,
			path:    "api/v1/common-stats",
			handler: statsHandlers.GetCommonStats,
		},
		Route{
			Method:  GET,
			path:    "api/v1/transactions-history-chart",
			handler: statsHandlers.GetTransactionsHistoryChart,
		},
		Route{
			Method:  GET,
			path:    "api/v1/transactions-history",
			handler: statsHandlers.GetTransactionsHistory,
		},
		Route{
			Method:  GET,
			path:    "api/v1/active-addresses-history",
			handler: statsHandlers.GetActiveAddressesHistory,
		},
		Route{
			Method:  GET,
			path:    "api/v1/total-addresses-growth",
			handler: statsHandlers.GetTotalAddressesGrowth,
		},
		Route{
			Method:  GET,
			path:    "api/v1/gas-used-history",
			handler: statsHandlers.GetGasUsedHistory,
		},
		Route{
			Method:  GET,
			path:    "api/v1/total-fee-history",
			handler: statsHandlers.GetTotalFeeHistory,
		},
	)

	statusHandlers := httpapi_handlers.NewStatusHandler(
		logger,
		cosmosAppClient,
		*blockscoutClient,
		rdbConn.ToHandle(),
	)
	routes = append(routes,
		Route{
			Method:  GET,
			path:    "api/v1/status",
			handler: statusHandlers.GetStatus,
		},
	)

	transactionHandler := httpapi_handlers.NewTransactions(logger, *blockscoutClient, rdbConn.ToHandle())
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
		Route{
			Method:  GET,
			path:    "api/v1/transactions/internal-transactions/{hash}",
			handler: transactionHandler.ListInternalTransactionsByHash,
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
