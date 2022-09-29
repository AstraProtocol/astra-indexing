package command

import (
	entity_event "github.com/AstraProtocol/astra-indexing/entity/event"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	"github.com/AstraProtocol/astra-indexing/usecase/model"
)

type CreateMsgRevokeAllowance struct {
	msgCommonParams event.MsgCommonParams
	params          model.MsgRevokeAllowanceParams
}

func NewCreateMsgRevokeAllowance(
	msgCommonParams event.MsgCommonParams,
	params model.MsgRevokeAllowanceParams,
) *CreateMsgRevokeAllowance {
	return &CreateMsgRevokeAllowance{
		msgCommonParams,
		params,
	}
}

func (*CreateMsgRevokeAllowance) Name() string {
	return "/cosmos.feegrant.v1beta1.MsgRevokeAllowance.Create"
}

func (*CreateMsgRevokeAllowance) Version() int {
	return 1
}

func (cmd *CreateMsgRevokeAllowance) Exec() (entity_event.Event, error) {
	event := event.NewMsgRevokeAllowance(cmd.msgCommonParams, cmd.params)
	return event, nil
}
