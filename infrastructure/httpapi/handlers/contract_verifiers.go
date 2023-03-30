package handlers

import (
	"errors"
	"strconv"
	"time"

	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	blockscout_infrastructure "github.com/AstraProtocol/astra-indexing/infrastructure/blockscout"
	"github.com/AstraProtocol/astra-indexing/infrastructure/httpapi"
	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"
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

func (handler *ContractVerifiers) Verify(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "Verify"
	// handle api's params

	module := string(ctx.PostArgs().Peek("module"))
	if module != "contract" {
		handler.logger.Errorf("invalid %s address params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "POST", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid module"))
		return
	}

	action := string(ctx.PostArgs().Peek("action"))

	bodyParams := make(map[string]string)

	// required params
	bodyParams["module"] = module
	bodyParams["action"] = action
	//

	switch action {
	case "verifysourcecode":
		// required params
		bodyParams["codeformat"] = string(ctx.PostArgs().Peek("codeformat"))
		bodyParams["contractaddress"] = string(ctx.PostArgs().Peek("contractaddress"))
		bodyParams["contractname"] = string(ctx.PostArgs().Peek("contractname"))
		bodyParams["compilerversion"] = string(ctx.PostArgs().Peek("compilerversion"))
		bodyParams["sourceCode"] = string(ctx.PostArgs().Peek("sourceCode"))
		//

		bodyParams["constructorArguements"] = string(ctx.PostArgs().Peek("constructorArguements"))
		bodyParams["autodetectConstructorArguments"] = string(ctx.PostArgs().Peek("autodetectConstructorArguments"))

		handler.verifySourceCode(ctx, bodyParams, startTime)
	default:
		handler.logger.Errorf("invalid %s address params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "POST", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("not implemented"))
		return
	}
}

func (handler *ContractVerifiers) verifySourceCode(ctx *fasthttp.RequestCtx, rawBody interface{}, startTime time.Time) {
	recordMethod := "VerifySourceCode"

	resp, err := handler.blockscoutClient.Verify(rawBody)
	if err != nil {
		handler.logger.Errorf("error verifying source code from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "POST", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "POST", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, resp)
}
