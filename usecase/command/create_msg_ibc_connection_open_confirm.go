package command

import (
	entity_event "github.com/AstraProtocol/astra-indexing/entity/event"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	ibc_model "github.com/AstraProtocol/astra-indexing/usecase/model/ibc"
)

type CreateMsgIBCConnectionOpenConfirm struct {
	msgCommonParams event.MsgCommonParams
	params          ibc_model.MsgConnectionOpenConfirmParams
}

func NewCreateMsgIBCConnectionOpenConfirm(
	msgCommonParams event.MsgCommonParams,
	params ibc_model.MsgConnectionOpenConfirmParams,
) *CreateMsgIBCConnectionOpenConfirm {
	return &CreateMsgIBCConnectionOpenConfirm{
		msgCommonParams,
		params,
	}
}

func (*CreateMsgIBCConnectionOpenConfirm) Name() string {
	return "/ibc.core.connection.v1.MsgConnectionOpenConfirm.Create"
}

func (*CreateMsgIBCConnectionOpenConfirm) Version() int {
	return 1
}

func (cmd *CreateMsgIBCConnectionOpenConfirm) Exec() (entity_event.Event, error) {
	event := event.NewMsgIBCConnectionOpenConfirm(cmd.msgCommonParams, cmd.params)
	return event, nil
}
