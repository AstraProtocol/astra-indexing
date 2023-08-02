package handlers

import (
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	jsonrpc_infrastructure "github.com/AstraProtocol/astra-indexing/infrastructure/jsonrpc"
	"github.com/valyala/fasthttp"
)

type Jsonrpc struct {
	logger        applogger.Logger
	jsonrpcClient jsonrpc_infrastructure.HTTPClient
}

func NewJsonrpc(
	logger applogger.Logger,
	jsonrpcClient jsonrpc_infrastructure.HTTPClient,
) *Jsonrpc {
	return &Jsonrpc{
		logger.WithFields(applogger.LogFields{
			"module": "JsonRPCHandler",
		}),
		jsonrpcClient,
	}
}

func (handler *Jsonrpc) TokenPrice(ctx *fasthttp.RequestCtx) {

}
