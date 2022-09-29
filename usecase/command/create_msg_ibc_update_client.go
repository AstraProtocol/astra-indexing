package command

import (
	entity_event "github.com/AstraProtocol/astra-indexing/entity/event"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	ibc_model "github.com/AstraProtocol/astra-indexing/usecase/model/ibc"
)

type CreateMsgIBCUpdateClient struct {
	msgCommonParams event.MsgCommonParams
	params          ibc_model.MsgUpdateClientParams
}

func NewCreateMsgIBCUpdateClient(
	msgCommonParams event.MsgCommonParams,
	params ibc_model.MsgUpdateClientParams,
) *CreateMsgIBCUpdateClient {
	return &CreateMsgIBCUpdateClient{
		msgCommonParams,
		params,
	}
}

func (*CreateMsgIBCUpdateClient) Name() string {
	return "/ibc.core.client.v1.MsgUpdateClient.Create"
}

func (*CreateMsgIBCUpdateClient) Version() int {
	return 1
}

func (cmd *CreateMsgIBCUpdateClient) Exec() (entity_event.Event, error) {
	event := event.NewMsgIBCUpdateClient(cmd.msgCommonParams, cmd.params)
	return event, nil
}
