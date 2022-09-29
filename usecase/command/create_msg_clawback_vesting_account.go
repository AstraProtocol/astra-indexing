package command

import (
	entity_event "github.com/AstraProtocol/astra-indexing/entity/event"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	"github.com/AstraProtocol/astra-indexing/usecase/model"
)

type CreateClawbackVestingAccount struct {
	msgCommonParams event.MsgCommonParams
	params          model.MsgCreateClawbackVestingAccountParams
}

func NewCreateClawbackVestingAccount(
	msgCommonParams event.MsgCommonParams,
	params model.MsgCreateClawbackVestingAccountParams,
) *CreateClawbackVestingAccount {
	return &CreateClawbackVestingAccount{
		msgCommonParams,
		params,
	}
}

// Name returns name of command
func (*CreateClawbackVestingAccount) Name() string {
	return "/evmos.vesting.v1.MsgCreateClawbackVestingAccount.Create"
}

// Version returns version of command
func (*CreateClawbackVestingAccount) Version() int {
	return 1
}

// Exec process the command data and return the event accordingly
func (cmd *CreateClawbackVestingAccount) Exec() (entity_event.Event, error) {
	event := event.NewMsgCreateClawbackVestingAccount(cmd.msgCommonParams, cmd.params)
	return event, nil
}
