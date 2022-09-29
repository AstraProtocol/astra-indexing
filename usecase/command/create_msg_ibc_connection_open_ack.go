package command

import (
	entity_event "github.com/AstraProtocol/astra-indexing/entity/event"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	ibc_model "github.com/AstraProtocol/astra-indexing/usecase/model/ibc"
)

type CreateMsgIBCConnectionOpenAck struct {
	msgCommonParams event.MsgCommonParams
	params          ibc_model.MsgConnectionOpenAckParams
}

func NewCreateMsgIBCConnectionOpenAck(
	msgCommonParams event.MsgCommonParams,
	params ibc_model.MsgConnectionOpenAckParams,
) *CreateMsgIBCConnectionOpenAck {
	return &CreateMsgIBCConnectionOpenAck{
		msgCommonParams,
		params,
	}
}

func (*CreateMsgIBCConnectionOpenAck) Name() string {
	return "/ibc.core.connection.v1.MsgConnectionOpenAck.Create"
}

func (*CreateMsgIBCConnectionOpenAck) Version() int {
	return 1
}

func (cmd *CreateMsgIBCConnectionOpenAck) Exec() (entity_event.Event, error) {
	event := event.NewMsgIBCConnectionOpenAck(cmd.msgCommonParams, cmd.params)
	return event, nil
}
