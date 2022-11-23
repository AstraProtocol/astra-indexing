package command

import entity_event "github.com/AstraProtocol/astra-indexing/entity/event"

type Command interface {
	Name() string
	Version() int

	// Exec process the command data and return the event accordingly
	// Currently one command will generates only one event
	Exec() (entity_event.Event, error)
}
