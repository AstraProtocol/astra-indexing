package command

import (
	entity_event "github.com/AstraProtocol/astra-indexing/entity/event"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	"github.com/AstraProtocol/astra-indexing/usecase/model"
)

type CreateMint struct {
	blockHeight int64
	params      model.MintParams
}

func NewCreateMint(blockHeight int64, params model.MintParams) *CreateMint {
	return &CreateMint{
		blockHeight,
		params,
	}
}

// Name returns name of command
func (*CreateMint) Name() string {
	return "CreateMint"
}

// Version returns version of command
func (*CreateMint) Version() int {
	return 1
}

// Exec process the command data and return the event accordingly
func (cmd *CreateMint) Exec() (entity_event.Event, error) {
	event := event.NewMinted(cmd.blockHeight, cmd.params)
	return event, nil
}
