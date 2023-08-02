package handlers

import (
	"errors"
	"math/big"
	"strconv"
	"strings"
	"time"

	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/AstraProtocol/astra-indexing/infrastructure/httpapi"
	jsonrpc_infrastructure "github.com/AstraProtocol/astra-indexing/infrastructure/jsonrpc"
	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"
	"github.com/valyala/fasthttp"
)

type JsonRPC struct {
	logger        applogger.Logger
	jsonrpcClient jsonrpc_infrastructure.HTTPClient
}

func NewJsonRPC(
	logger applogger.Logger,
	jsonrpcClient jsonrpc_infrastructure.HTTPClient,
) *JsonRPC {
	return &JsonRPC{
		logger.WithFields(applogger.LogFields{
			"module": "JsonRPCHandler",
		}),
		jsonrpcClient,
	}
}

func (handler *JsonRPC) GetTokenPrice(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetTokenPrice"

	// handle api's params
	addressHash, contractParamOk := URLValueGuard(ctx, handler.logger, "contractaddress")
	if !contractParamOk {
		handler.logger.Errorf("invalid %s params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid contractaddress param"))
		return
	}

	selector, selectorParamOk := URLValueGuard(ctx, handler.logger, "selector")
	if !selectorParamOk {
		handler.logger.Errorf("invalid %s params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid selector param"))
		return
	}
	//

	payload := make(map[string]interface{})
	payload["id"] = 1
	payload["jsonrpc"] = "2.0"
	payload["method"] = "eth_call"

	params := make([]interface{}, 0)
	paramPayload := make(map[string]string)
	paramPayload["data"] = selector
	paramPayload["to"] = addressHash
	params = append(params, paramPayload)
	params = append(params, "latest")

	payload["params"] = params

	response, err := handler.jsonrpcClient.EthCall(payload)
	if err != nil {
		handler.logger.Errorf("error fetching token price from RPC: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	//parse token price
	priceData := strings.Split(response.Result.(string), "x")[1]
	reserve0 := new(big.Int)
	reserve0.SetString(priceData[0:64], 16)

	reserve1 := new(big.Int)
	reserve1.SetString(priceData[64:128], 16)

	blockTimestampLast := new(big.Int)
	blockTimestampLast.SetString(priceData[128:], 16)

	//price = reserve0/reserve1
	price := new(big.Float).Quo(big.NewFloat(0).SetInt(reserve0), big.NewFloat(0).SetInt(reserve1))

	result := make(map[string]string)
	result["price"] = price.String()
	result["blockTimestampLast"] = blockTimestampLast.String()

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, result)
}
