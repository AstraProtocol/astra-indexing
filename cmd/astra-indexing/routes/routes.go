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
	searchHandler := httpapi_handlers.NewSearch(logger, *blockscoutClient, cosmosAppClient, rdbConn.ToHandle())
	routes = append(routes,
		Route{
			Method:  GET,
			path:    "api/v1/search",
			handler: searchHandler.Search,
		},
	)

	blocksHandler := httpapi_handlers.NewBlocks(logger, rdbConn.ToHandle(), cosmosAppClient, *blockscoutClient)
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
		Route{
			Method:  GET,
			path:    "api/v1/blocks/raw-txs/{height}",
			handler: blocksHandler.ListRawTxsByHeight,
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
			path:    "api/v1/accounts/detail/{account}",
			handler: accountsHandlers.GetDetailAddress,
		},
		Route{
			Method:  GET,
			path:    "api/v1/accounts/getabi/{account}",
			handler: accountsHandlers.GetAbiByAddressHash,
		},
		Route{
			Method:  GET,
			path:    "api/v1/accounts/tokenlist/{account}",
			handler: accountsHandlers.GetTokensOfAnAddress,
		},
		Route{
			Method:  GET,
			path:    "api/v1/accounts/get-coin-balances-history/{account}",
			handler: accountsHandlers.GetCoinBalancesHistory,
		},
		Route{
			Method:  GET,
			path:    "api/v1/accounts/{account}/coin-balances/by-day",
			handler: accountsHandlers.AddressCoinBalancesByDate,
		},
	)

	contractsHandler := httpapi_handlers.NewContracts(
		logger,
		*blockscoutClient,
	)
	routes = append(routes,
		Route{
			Method:  GET,
			path:    "api/v1/contract/get-list-tokens",
			handler: contractsHandler.GetListTokens,
		},
		Route{
			Method:  GET,
			path:    "api/v1/contract/token-transfers/{contractaddress}",
			handler: contractsHandler.GetListTokenTransfersByContractAddressHash,
		},
		Route{
			Method:  GET,
			path:    "api/v1/contract/txs/{contractaddress}",
			handler: contractsHandler.GetListTxsByContractAddressHash,
		},
		Route{
			Method:  GET,
			path:    "api/v1/contract/deposit-txs/{contractaddress}",
			handler: contractsHandler.GetListDepositTxsByContractAddressHash,
		},
		Route{
			Method:  GET,
			path:    "api/v1/contract/token-holders/{contractaddress}",
			handler: contractsHandler.GetListTokenHoldersOfAContractAddressHash,
		},
		Route{
			Method:  GET,
			path:    "api/v1/contract/token-inventory/{contractaddress}",
			handler: contractsHandler.GetTokenInventoryOfAContractAddressHash,
		},
		Route{
			Method:  GET,
			path:    "api/v1/contract/token-transfers-by-tokenid/contractaddress={contractaddress}/tokenid={tokenid}",
			handler: contractsHandler.GetTokenTransfersByTokenId,
		},
		Route{
			Method:  GET,
			path:    "api/v1/contract/source-code/{contractaddress}",
			handler: contractsHandler.GetSourceCodeOfAContractAddressHash,
		},
		Route{
			Method:  GET,
			path:    "api/v1/contract/token-detail/{contractaddress}",
			handler: contractsHandler.GetTokenDetail,
		},
		Route{
			Method:  GET,
			path:    "api/v1/contract/token-metadata/contractaddress={contractaddress}/tokenid={tokenid}",
			handler: contractsHandler.GetTokenMetadata,
		},
	)

	contractVerifiersHandler := httpapi_handlers.NewContractVerifiers(
		logger,
		*blockscoutClient,
	)
	routes = append(routes,
		Route{
			Method:  GET,
			path:    "api",
			handler: contractVerifiersHandler.ContractActions,
		},
		Route{
			Method:  POST,
			path:    "verify_smart_contract/contract_verifications",
			handler: contractVerifiersHandler.VerifyFlattened,
		},
		Route{
			Method:  POST,
			path:    "api",
			handler: contractVerifiersHandler.Verify,
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
		Route{
			Method:  GET,
			path:    "api/v1/accounts/internal-transactions/{account}",
			handler: accountTransactionsHandler.GetInternalTxsByAddressHash,
		},
		Route{
			Method:  GET,
			path:    "api/v1/accounts/token-transfers/{account}",
			handler: accountTransactionsHandler.GetListTokenTransfersByAddressHash,
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
		Route{
			Method:  GET,
			path:    "api/v1/market-history-chart",
			handler: statsHandlers.MarketHistoryChart,
		},
		Route{
			Method:  GET,
			path:    "api/v1/gas-price-oracle",
			handler: statsHandlers.GasPriceOracle,
		},
		Route{
			Method:  GET,
			path:    "api/v1/evm-versions",
			handler: statsHandlers.EvmVersions,
		},
		Route{
			Method:  GET,
			path:    "api/v1/compiler-versions/{compiler}",
			handler: statsHandlers.CompilerVersions,
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
		Route{
			Method:  GET,
			path:    "api/v2/transactions/internal-transactions/{hash}",
			handler: transactionHandler.ListInternalTransactionsByHashv2,
		},
		Route{
			Method:  GET,
			path:    "api/v1/transactions/getabi/{hash}",
			handler: transactionHandler.GetAbiByTransactionHash,
		},
		Route{
			Method:  GET,
			path:    "api/v1/transactions/getrawtrace/{hash}",
			handler: transactionHandler.GetRawTraceByTransactionHash,
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
