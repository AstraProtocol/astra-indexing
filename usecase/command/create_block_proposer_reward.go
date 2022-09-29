package command

import (
	entity_event "github.com/AstraProtocol/astra-indexing/entity/event"
	"github.com/AstraProtocol/astra-indexing/usecase/coin"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
)

type CreateBlockProposerReward struct {
	blockHeight int64
	validator   string
	amount      coin.DecCoins
}

func NewCreateBlockProposerReward(blockHeight int64, validator string, amount coin.DecCoins) *CreateBlockProposerReward {
	return &CreateBlockProposerReward{
		blockHeight,
		validator,
		amount,
	}
}

// Name returns name of command
func (*CreateBlockProposerReward) Name() string {
	return "CreateBlockProposerReward"
}

// Version returns version of command
func (*CreateBlockProposerReward) Version() int {
	return 1
}

// Exec process the command data and return the event accordingly
func (cmd *CreateBlockProposerReward) Exec() (entity_event.Event, error) {
	event := event.NewProposerRewarded(cmd.blockHeight, cmd.validator, cmd.amount)
	return event, nil
}
