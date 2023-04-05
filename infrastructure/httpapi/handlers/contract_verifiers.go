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
	if module != "contract" && module != "logs" {
		handler.logger.Errorf("%s: not implemented module %s", recordMethod, module)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "POST", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("not implemented module"))
		return
	}

	if module == "logs" {
		action := string(ctx.PostArgs().Peek("action"))
		bodyParams := make(map[string]string)
		// required params
		bodyParams["module"] = module
		bodyParams["action"] = action
		bodyParams["fromBlock"] = string(ctx.PostArgs().Peek("fromBlock"))
		bodyParams["toBlock"] = string(ctx.PostArgs().Peek("toBlock"))
		bodyParams["address"] = string(ctx.PostArgs().Peek("address"))
		bodyParams["topic0"] = string(ctx.PostArgs().Peek("topic0"))
		//

		for i := 1; i <= 3; i++ {
			topic := fmt.Sprintf("topic%d", i)
			if string(ctx.PostArgs().Peek(topic)) != "" {
				bodyParams[topic] = string(ctx.PostArgs().Peek(topic))
			}
		}
		for i := 0; i <= 2; i++ {
			for j := i + 1; j <= 3; j++ {
				topic_opr := fmt.Sprintf("topic%d_%d_opr", i, j)
				if string(ctx.PostArgs().Peek(topic_opr)) != "" {
					bodyParams[topic_opr] = string(ctx.PostArgs().Peek(topic_opr))
				}
			}
		}

		resp, err := handler.blockscoutClient.Logs(bodyParams)
		if err != nil {
			handler.logger.Errorf("error fetching logs from blockscout: %v", err)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "POST", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, err)
			return
		}

		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "POST", time.Since(startTime).Milliseconds())
		httpapi.SuccessNotWrappedResult(ctx, resp)
	}

	if module == "contract" {
		action := string(ctx.PostArgs().Peek("action"))
		bodyParams := make(map[string]string)
		// required params
		bodyParams["module"] = module
		bodyParams["action"] = action
		if action == "verifyproxycontract" {
			action = "verifysourcecode"
		}
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
			handler.logger.Errorf("%s: %s not implemented", recordMethod, action)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "POST", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, fmt.Errorf("%s: %s not implemented", recordMethod, action))
			return
		}
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

	bodyParams := make(map[string](map[string]string))

	smartContractParams := make(map[string]string)
	smartContractParams["address_hash"] = string(ctx.PostArgs().Peek("smart_contract[address_hash]"))
	smartContractParams["name"] = string(ctx.PostArgs().Peek("smart_contract[name]"))
	smartContractParams["nightly_builds"] = string(ctx.PostArgs().Peek("smart_contract[nightly_builds]"))
	smartContractParams["compiler_version"] = string(ctx.PostArgs().Peek("smart_contract[compiler_version]"))
	smartContractParams["evm_version"] = string(ctx.PostArgs().Peek("smart_contract[evm_version]"))
	smartContractParams["optimization"] = string(ctx.PostArgs().Peek("smart_contract[optimization]"))
	smartContractParams["contract_source_code"] = string(ctx.PostArgs().Peek("smart_contract[contract_source_code]"))
	smartContractParams["autodetect_constructor_args"] = string(ctx.PostArgs().Peek("smart_contract[autodetect_constructor_args]"))
	smartContractParams["constructor_arguments"] = string(ctx.PostArgs().Peek("smart_contract[constructor_arguments]"))

	externalLibrariesParams := make(map[string]string)
	for i := 1; i <= 10; i++ {
		libraryName := fmt.Sprintf("library%d_name", i)
		libraryAddress := fmt.Sprintf("library%d_address", i)
		externalLibrariesParams[libraryName] = string(ctx.PostArgs().Peek(fmt.Sprintf("external_libraries[%s]", libraryName)))
		externalLibrariesParams[libraryAddress] = string(ctx.PostArgs().Peek(fmt.Sprintf("external_libraries[%s]", libraryAddress)))
	}

	bodyParams["smart_contract"] = smartContractParams
	bodyParams["external_libraries"] = externalLibrariesParams

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

//checkverifystatus

func (handler *ContractVerifiers) ContractActions(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "CheckVerifyStatus"
	// handle api's params

	module := string(ctx.QueryArgs().Peek("module"))
	if module != "contract" {
		handler.logger.Errorf("%s: not implemented module %s", recordMethod, module)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "POST", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("not implemented module"))
		return
	}

	action := string(ctx.QueryArgs().Peek("action"))
	if action == "checkproxyverification" {
		action = "checkverifystatus"
	}
	if action != "checkverifystatus" && action != "getsourcecode" {
		handler.logger.Errorf("%s: invalid action %s", recordMethod, action)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "POST", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid action"))
		return
	}

	switch action {
	case "checkverifystatus":
		guid := string(ctx.QueryArgs().Peek("guid"))
		if guid == "" {
			handler.logger.Errorf("invalid guid param: %s", recordMethod)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("invalid guid param"))
			return
		}
		verifyStatus, err := handler.blockscoutClient.CheckVerifyStatus(guid)
		if err != nil {
			handler.logger.Errorf("error fetching verify status from blockscout: %v", err)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, err)
			return
		}
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.SuccessNotWrappedResult(ctx, verifyStatus)
	case "getsourcecode":
		addressHash := string(ctx.QueryArgs().Peek("address"))
		if addressHash == "" {
			handler.logger.Errorf("invalid address hash param: %s", recordMethod)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("invalid address hash param"))
			return
		}
		sourceCode, err := handler.blockscoutClient.GetSourceCode(addressHash)
		if err != nil {
			handler.logger.Errorf("error fetching source code from blockscout: %v", err)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, err)
			return
		}
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.SuccessNotWrappedResult(ctx, sourceCode)
	default:
		handler.logger.Errorf("%s: %s not implemented", recordMethod, action)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, fmt.Errorf("%s: %s not implemented", recordMethod, action))
		return
	}
}
