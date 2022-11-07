package command

import (
	entity_event "github.com/AstraProtocol/astra-indexing/entity/event"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	"github.com/AstraProtocol/astra-indexing/usecase/model"
)

type CreateMsgWithdrawValidatorCommission struct {
	msgCommonParams event.MsgCommonParams

	params model.MsgWithdrawValidatorCommissionParams
}

func NewCreateMsgWithdrawValidatorCommission(
	msgCommonParams event.MsgCommonParams,
	params model.MsgWithdrawValidatorCommissionParams,
) *CreateMsgWithdrawValidatorCommission {
	return &CreateMsgWithdrawValidatorCommission{
		msgCommonParams,

		params,
	}
}

func (_ *CreateMsgWithdrawValidatorCommission) Name() string {
	return "/cosmos.distribution.v1beta1.MsgWithdrawValidatorCommission.Create"
}

func (_ *CreateMsgWithdrawValidatorCommission) Version() int {
	return 1
}

func (cmd *CreateMsgWithdrawValidatorCommission) Exec() (entity_event.Event, error) {
	eventCommission := event.NewMsgWithdrawValidatorCommission(
		cmd.msgCommonParams,
		cmd.params,
	)
	return eventCommission, nil
}
