package command

import (
	entity_event "github.com/AstraProtocol/astra-indexing/entity/event"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	"github.com/AstraProtocol/astra-indexing/usecase/model"
)

type CreateAccountTransfer struct {
	blockHeight int64
	params      model.AccountTransferParams
}

func NewCreateAccountTransfer(
	blockHeight int64,
	params model.AccountTransferParams,
) *CreateAccountTransfer {
	return &CreateAccountTransfer{
		blockHeight,
		params,
	}
}

// Name returns name of command
func (*CreateAccountTransfer) Name() string {
	return "CreateAccountTransfer"
}

// Version returns version of command
func (*CreateAccountTransfer) Version() int {
	return 1
}

// Exec process the command data and return the event accordingly
func (cmd *CreateAccountTransfer) Exec() (entity_event.Event, error) {
	event := event.NewAccountTransferred(cmd.blockHeight, cmd.params)
	return event, nil
}
