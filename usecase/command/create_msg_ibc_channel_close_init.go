package command

import (
	entity_event "github.com/AstraProtocol/astra-indexing/entity/event"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	ibc_model "github.com/AstraProtocol/astra-indexing/usecase/model/ibc"
)

type CreateMsgIBCChannelCloseInit struct {
	msgCommonParams event.MsgCommonParams
	params          ibc_model.MsgChannelCloseInitParams
}

func NewCreateMsgIBCChannelCloseInit(
	msgCommonParams event.MsgCommonParams,
	params ibc_model.MsgChannelCloseInitParams,
) *CreateMsgIBCChannelCloseInit {
	return &CreateMsgIBCChannelCloseInit{
		msgCommonParams,
		params,
	}
}

func (*CreateMsgIBCChannelCloseInit) Name() string {
	return "/ibc.core.channel.v1.MsgChannelCloseInit.Create"
}

func (*CreateMsgIBCChannelCloseInit) Version() int {
	return 1
}

func (cmd *CreateMsgIBCChannelCloseInit) Exec() (entity_event.Event, error) {
	event := event.NewMsgIBCChannelCloseInit(cmd.msgCommonParams, cmd.params)
	return event, nil
}
