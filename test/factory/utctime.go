package factory

import (
	"math/rand"

	"github.com/AstraProtocol/astra-indexing/external/utctime"
	random "github.com/brianvoe/gofakeit/v5"
)

func addUTCTimeFuncLookup() {
	random.AddFuncLookup("utctime", random.Info{
		Category:    "custom",
		Description: "Random time.Time",
		Example:     "0",
		Output:      "utctime.UTCTime",
		Call: func(m *map[string][]string, info *random.Info) (interface{}, error) {
			return RandomUTCTime(), nil
		},
	})
}

func RandomUTCTime() utctime.UTCTime {
	// nolint:gosec
	return utctime.FromUnixNano(rand.Int63())
}
