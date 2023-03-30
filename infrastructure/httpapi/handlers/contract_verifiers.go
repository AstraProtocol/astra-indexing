package handlers

import (
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	blockscout_infrastructure "github.com/AstraProtocol/astra-indexing/infrastructure/blockscout"
	"github.com/valyala/fasthttp"
)

type ContractVerifiers struct {
	logger           applogger.Logger
	blockscoutClient blockscout_infrastructure.HTTPClient
}

func NewContractVerifiers(
	logger applogger.Logger,
	blockscoutClient blockscout_infrastructure.HTTPClient,
) *ContractVerifiers {
	return &ContractVerifiers{
		logger.WithFields(applogger.LogFields{
			"module": "ContractVerifiersHandler",
		}),
		blockscoutClient,
	}
}

func (handler *ContractVerifiers) VerifySourceCode(ctx *fasthttp.RequestCtx) {
	//TODO: implement verify smart contract with standard json input
}
