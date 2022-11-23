package command

import (
	entity_event "github.com/AstraProtocol/astra-indexing/entity/event"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	"github.com/AstraProtocol/astra-indexing/usecase/model/genesis"
)

type CreateGenesisValidator struct {
	params genesis.CreateGenesisValidatorParams
}

func NewCreateGenesisValidator(
	params genesis.CreateGenesisValidatorParams,
) *CreateGenesisValidator {
	return &CreateGenesisValidator{
		params,
	}
}

func (*CreateGenesisValidator) Name() string {
	return "CreateGenesisValidator"
}

func (*CreateGenesisValidator) Version() int {
	return 1
}

func (cmd *CreateGenesisValidator) Exec() (entity_event.Event, error) {
	event := event.NewCreateGenesisValidator(cmd.params)
	return event, nil
}
