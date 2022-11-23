package command

import (
	entity_event "github.com/AstraProtocol/astra-indexing/entity/event"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	ibc_model "github.com/AstraProtocol/astra-indexing/usecase/model/ibc"
)

type CreateMsgIBCChannelOpenInit struct {
	msgCommonParams event.MsgCommonParams
	params          ibc_model.MsgChannelOpenInitParams
}

func NewCreateMsgIBCChannelOpenInit(
	msgCommonParams event.MsgCommonParams,
	params ibc_model.MsgChannelOpenInitParams,
) *CreateMsgIBCChannelOpenInit {
	return &CreateMsgIBCChannelOpenInit{
		msgCommonParams,
		params,
	}
}

func (*CreateMsgIBCChannelOpenInit) Name() string {
	return "/ibc.core.channel.v1.MsgChannelOpenInit.Create"
}

func (*CreateMsgIBCChannelOpenInit) Version() int {
	return 1
}

func (cmd *CreateMsgIBCChannelOpenInit) Exec() (entity_event.Event, error) {
	event := event.NewMsgIBCChannelOpenInit(cmd.msgCommonParams, cmd.params)
	return event, nil
}
