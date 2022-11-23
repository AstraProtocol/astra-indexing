package eventhandler

import "github.com/AstraProtocol/astra-indexing/entity/event"

type Handler interface {
	GetLastHandledEventHeight() (*int64, error)

	HandleEvents(blockHeight int64, events []event.Event) error

	Id() string
}
