package handlers

import (
	"errors"
	"fmt"
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
	httpapi.SuccessNotWrappedResult(ctx, resp)
}

func (handler *ContractVerifiers) VerifyFlattened(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "VerifyFlattened"
	// handle api's params

	bodyParams := make(map[string]string)

	bodyParams["smart_contract[address_hash]"] = string(ctx.PostArgs().Peek("smart_contract[address_hash]"))
	bodyParams["smart_contract[name]"] = string(ctx.PostArgs().Peek("smart_contract[name]"))
	bodyParams["smart_contract[nightly_builds]"] = string(ctx.PostArgs().Peek("smart_contract[nightly_builds]"))
	bodyParams["smart_contract[compiler_version]"] = string(ctx.PostArgs().Peek("smart_contract[compiler_version]"))
	bodyParams["smart_contract[evm_version]"] = string(ctx.PostArgs().Peek("smart_contract[evm_version]"))
	bodyParams["smart_contract[optimization]"] = string(ctx.PostArgs().Peek("smart_contract[optimization]"))
	bodyParams["smart_contract[contract_source_code]"] = string(ctx.PostArgs().Peek("smart_contract[contract_source_code]"))
	bodyParams["smart_contract[autodetect_constructor_args]"] = string(ctx.PostArgs().Peek("smart_contract[autodetect_constructor_args]"))
	bodyParams["smart_contract[constructor_arguments]"] = string(ctx.PostArgs().Peek("smart_contract[constructor_arguments]"))

	for i := 1; i <= 10; i++ {
		libraryName := fmt.Sprintf("external_libraries[library%d_name]", i)
		libraryAddress := fmt.Sprintf("external_libraries[library%d_address]", i)
		bodyParams[libraryName] = string(ctx.PostArgs().Peek(libraryName))
		bodyParams[libraryAddress] = string(ctx.PostArgs().Peek(libraryAddress))
	}

	resp, err := handler.blockscoutClient.VerifyFlattened(bodyParams)
	if err != nil {
		handler.logger.Errorf("error verifying flattened source code from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "POST", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "POST", time.Since(startTime).Milliseconds())
	httpapi.SuccessNotWrappedResult(ctx, resp)
}
