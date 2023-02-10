package command

import (
	entity_event "github.com/AstraProtocol/astra-indexing/entity/event"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	"github.com/AstraProtocol/astra-indexing/usecase/model"
)

type CreateClawback struct {
	msgCommonParams event.MsgCommonParams
	params          model.MsgClawbackParams
}

func NewCreateClawback(
	msgCommonParams event.MsgCommonParams,
	params model.MsgClawbackParams,
) *CreateClawback {
	return &CreateClawback{
		msgCommonParams,
		params,
	}
}

// Name returns name of command
func (*CreateClawback) Name() string {
	return "/evmos.vesting.v1.MsgClawback.Create"
}

// Version returns version of command
func (*CreateClawback) Version() int {
	return 1
}

// Exec process the command data and return the event accordingly
func (cmd *CreateClawback) Exec() (entity_event.Event, error) {
	event := event.NewMsgClawback(cmd.msgCommonParams, cmd.params)
	return event, nil
}
