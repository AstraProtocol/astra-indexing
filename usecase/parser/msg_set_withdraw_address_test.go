package parser_test

import (
	"github.com/AstraProtocol/astra-indexing/usecase/model"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/AstraProtocol/astra-indexing/entity/command"
	command_usecase "github.com/AstraProtocol/astra-indexing/usecase/command"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	"github.com/AstraProtocol/astra-indexing/usecase/parser"
	usecase_parser_test "github.com/AstraProtocol/astra-indexing/usecase/parser/test"
)

var _ = Describe("ParseMsgCommands", func() {
	Describe("MsgSetWithdrawAddress", func() {
		It("should parse Msg commands when there is distribution.MsgSetWithdrawAddress in the transaction", func() {
			block, _ := mustParseBlockResp(usecase_parser_test.TX_MSG_SET_WITHDRAW_ADDRESS_BLOCK_RESP)
			blockResults := mustParseBlockResultsResp(
				usecase_parser_test.TX_MSG_SET_WITHDRAW_ADDRESS_BLOCK_RESULTS_RESP,
			)

			tx := mustParseTxsResp(usecase_parser_test.TX_MSG_SET_WITHDRAW_ADDRESS_TXS_RESP)
			txs := []model.Tx{*tx}

			accountAddressPrefix := "tcro"
			bondingDenom := "basetcro"

			pm := usecase_parser_test.InitParserManager()

			cmds, possibleSignerAddresses, err := parser.ParseBlockTxsMsgToCommands(
				pm,
				block.Height,
				blockResults,
				txs,
				accountAddressPrefix,
				bondingDenom,
			)
			Expect(err).To(BeNil())
			Expect(cmds).To(HaveLen(1))
			Expect(cmds).To(Equal([]command.Command{command_usecase.NewCreateMsgSetWithdrawAddress(
				event.MsgCommonParams{
					BlockHeight: int64(460060),
					TxHash:      "9C2501310E18EE69A7FE5CA1A684A0701C43BEB1A8D91EDA80CC598C924F9CBE",
					TxSuccess:   true,
					MsgIndex:    0,
				},
				model.MsgSetWithdrawAddressParams{
					DelegatorAddress: "tcro1fmprm0sjy6lz9llv7rltn0v2azzwcwzvk2lsyn",
					WithdrawAddress:  "tcro14m5a4kxt2e82uqqs5gtqza29dm5wqzya2jw9sh",
				},
			)}))
			Expect(possibleSignerAddresses).To(Equal([]string{"tcro1fmprm0sjy6lz9llv7rltn0v2azzwcwzvk2lsyn"}))
		})
	})
})
