package command

import (
	entity_event "github.com/AstraProtocol/astra-indexing/entity/event"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	"github.com/AstraProtocol/astra-indexing/usecase/model"
)

// CreateMsgDelegate is a command to create MsgDelegate event
type CreateMsgDelegate struct {
	msgCommonParams event.MsgCommonParams
	params          model.MsgDelegateParams
}

// NewCreateMsgDelegate create a new instance of CreateMsgDelegate command
func NewCreateMsgDelegate(msgCommonParams event.MsgCommonParams, params model.MsgDelegateParams) *CreateMsgDelegate {
	return &CreateMsgDelegate{
		msgCommonParams,
		params,
	}
}

// Name returns name of command
func (*CreateMsgDelegate) Name() string {
	return "/cosmos.staking.v1beta1.MsgDelegate.Create"
}

// Version returns version of command
func (*CreateMsgDelegate) Version() int {
	return 1
}

// Exec process the command data and return the event accordingly
func (cmd *CreateMsgDelegate) Exec() (entity_event.Event, error) {
	event := event.NewMsgDelegate(cmd.msgCommonParams, cmd.params)
	return event, nil
}
