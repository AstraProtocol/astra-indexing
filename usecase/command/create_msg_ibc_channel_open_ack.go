package command

import (
	entity_event "github.com/AstraProtocol/astra-indexing/entity/event"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	ibc_model "github.com/AstraProtocol/astra-indexing/usecase/model/ibc"
)

type CreateMsgIBCChannelOpenAck struct {
	msgCommonParams event.MsgCommonParams
	params          ibc_model.MsgChannelOpenAckParams
}

func NewCreateMsgIBCChannelOpenAck(
	msgCommonParams event.MsgCommonParams,
	params ibc_model.MsgChannelOpenAckParams,
) *CreateMsgIBCChannelOpenAck {
	return &CreateMsgIBCChannelOpenAck{
		msgCommonParams,
		params,
	}
}

func (*CreateMsgIBCChannelOpenAck) Name() string {
	return "/ibc.core.channel.v1.MsgChannelOpenAck.Create"
}

func (*CreateMsgIBCChannelOpenAck) Version() int {
	return 1
}

func (cmd *CreateMsgIBCChannelOpenAck) Exec() (entity_event.Event, error) {
	event := event.NewMsgIBCChannelOpenAck(cmd.msgCommonParams, cmd.params)
	return event, nil
}
