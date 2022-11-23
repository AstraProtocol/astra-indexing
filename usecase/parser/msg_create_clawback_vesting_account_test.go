package parser_test

import (
	"regexp"
	"strings"

	"github.com/AstraProtocol/astra-indexing/external/json"
	"github.com/AstraProtocol/astra-indexing/usecase/model"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/AstraProtocol/astra-indexing/infrastructure/tendermint"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	"github.com/AstraProtocol/astra-indexing/usecase/parser"
	usecase_parser_test "github.com/AstraProtocol/astra-indexing/usecase/parser/test"
)

var _ = Describe("ParseMsgCommands", func() {
	Describe("MsgExec", func() {
		It("should parse Msg commands when there is MsgExec in the transaction", func() {
			expected := `{
            "name": "/evmos.vesting.v1.MsgCreateClawbackVestingAccount.Created",
            "version": 1,
            "height": 2282704,
            "uuid": "{UUID}",
            "msgName": "/evmos.vesting.v1.MsgCreateClawbackVestingAccount",
            "txHash": "02ABB4448F7D351DD7DD1BC192ADECE2E7E1C7735F40AB83F4F86D7A6F706E58",
            "msgIndex": 0,
            "params": {
                "@type": "/evmos.vesting.v1.MsgCreateClawbackVestingAccount",
                "from_address": "astra16mqptvptnds4098cmdmz846lmazenegc08wwf7",
                "to_address": "astra1wxru954y35y88x2u8q2vsmmsazs7h3ld0yfnh6",
				"start_time": "1990-09-01T00:00:00Z",
				"lockup_periods": [
				  ],
				  "vesting_periods": [
					{
					  "amount": [
						{
						  "denom": "aastra",
						  "amount": "5000000000000000000"
						}
					  ],
					  "length": "1649350800"
					},
					{
					  "amount": [
						{
						  "denom": "aastra",
						  "amount": "5000000000000000000"
						}
					  ],
					  "length": "604800000"
					}
				  ],
				"merge": false
            }
        }`

			block, _, _ := tendermint.ParseBlockResp(strings.NewReader(
				usecase_parser_test.TX_MSG_CREATE_CLAWBACK_VESTING_ACCOUNT_BLOCK_RESP,
			))
			blockResults, _ := tendermint.ParseBlockResultsResp(strings.NewReader(
				usecase_parser_test.TX_MSG_CREATE_CLAWBACK_VESTING_ACCOUNT_BLOCK_RESULTS_RESP,
			))

			tx := mustParseTxsResp(usecase_parser_test.TX_MSG_CREATE_CLAWBACK_VESTING_ACCOUNT_TXS_RESP)
			txs := []model.Tx{*tx}

			accountAddressPrefix := "astra"
			stakingDenom := "aastra"

			pm := usecase_parser_test.InitParserManager()

			cmds, possibleSignerAddresses, err := parser.ParseBlockTxsMsgToCommands(
				pm,
				block.Height,
				blockResults,
				txs,
				accountAddressPrefix,
				stakingDenom,
			)
			Expect(err).To(BeNil())
			Expect(cmds).To(HaveLen(1))
			cmd := cmds[0]
			Expect(cmd.Name()).To(Equal("/evmos.vesting.v1.MsgCreateClawbackVestingAccount.Create"))

			untypedEvent, _ := cmd.Exec()
			createMsgCreateClawbackVestingAccount := untypedEvent.(*event.MsgCreateClawbackVestingAccount)

			regex, _ := regexp.Compile("\n?\r?\\s?")
			Expect(json.MustMarshalToString(createMsgCreateClawbackVestingAccount)).To(Equal(
				strings.Replace(
					regex.ReplaceAllString(expected, ""),
					"{UUID}",
					createMsgCreateClawbackVestingAccount.UUID(),
					-1,
				),
			))
			Expect(possibleSignerAddresses).To(Equal([]string{"astra16mqptvptnds4098cmdmz846lmazenegc08wwf7"}))
		})
	})
})
