package event_test

import (
	event_entity "github.com/AstraProtocol/astra-indexing/entity/event"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	event_usecase "github.com/AstraProtocol/astra-indexing/usecase/event"
	"github.com/AstraProtocol/astra-indexing/usecase/model"
	vesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("Create crawback vesting", func() {
	registry := event_entity.NewRegistry()
	event_usecase.RegisterEvents(registry)

	Describe("En/DecodeNewMsgCreateClawbackVestingAccount", func() {
		It("should able to encode and decode to the same event", func() {
			rawMsg := model.RawMsgCreateClawbackVestingAccount{
				FromAddress:    "astra1wxru954y35y88x2u8q2vsmmsazs7h3ld0yfnh6",
				ToAddress:      "astra1wxru954y35y88x2u8q2vsmmsazs7h3ld0yfnh6",
				StartTime:      time.UnixMicro(0),
				VestingPeriods: vesting.Periods{},
				LockupPeriods:  vesting.Periods{},
				Merge:          true}
			event := event_usecase.NewMsgCreateClawbackVestingAccount(
				event.MsgCommonParams{
					BlockHeight: int64(503978),
					TxHash:      "E69985AC8168383A81B7952DBE03EB9B3400FF80AEC0F362369DD7F38B1C2FE9",
					TxSuccess:   true,
					MsgIndex:    0,
				},
				model.MsgCreateClawbackVestingAccountParams{
					RawMsgCreateClawbackVestingAccount: rawMsg,
				},
			)

			encoded, err := event.ToJSON()
			Expect(err).To(BeNil())

			decodedEvent, err := registry.DecodeByType(
				event_usecase.MSG_CREATE_CRAW_BACK_VESTING_ACCOUNT_CREATED, 1, []byte(encoded),
			)
			Expect(err).To(BeNil())
			Expect(decodedEvent).To(Equal(event))
			typedEvent, _ := decodedEvent.(*event_usecase.MsgCreateClawbackVestingAccount)
			Expect(typedEvent.Name()).To(Equal(event_usecase.MSG_CREATE_CRAW_BACK_VESTING_ACCOUNT_CREATED))
			Expect(typedEvent.Version()).To(Equal(1))

			Expect(typedEvent.TxHash()).To(Equal(event.TxHash()))
			Expect(typedEvent.MsgIndex).To(Equal(event.MsgIndex))
			Expect(typedEvent.StartTime).To(Equal(event.StartTime))
			Expect(typedEvent.FromAddress).To(Equal(event.FromAddress))
			Expect(typedEvent.ToAddress).To(Equal(event.ToAddress))

		})
	})
})
