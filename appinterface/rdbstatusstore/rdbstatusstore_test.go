package rdbstatusstore_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdbstatusstore"
	"github.com/AstraProtocol/astra-indexing/external/primptr"
	"github.com/AstraProtocol/astra-indexing/infrastructure/pg"
)

var _ = Describe("RdbStatusStore", func() {
	WithTestPgxConn(func(pgxConn *pg.PgxConn, migrate rdb.Migrate) {
		BeforeEach(func() {
			_ = migrate.Reset()
			migrate.MustUp()
		})

		AfterEach(func() {
			_ = migrate.Reset()
		})

		Describe("GetLastIndexedBlockHeight", func() {
			It("should return the nil with no error on first run", func() {
				store := rdbstatusstore.NewRDbStatusStore(
					pgxConn.ToHandle(),
				)
				height, err := store.GetLastIndexedBlockHeight()

				Expect(err).To(BeNil())
				Expect(height).To(BeNil())
			})
		})

		Describe("UpdateLastIndexedBlockHeightWithRDbHandle", func() {
			It("should return the updated last indexed block height properly with no error", func() {
				store := rdbstatusstore.NewRDbStatusStore(
					pgxConn.ToHandle(),
				)

				newHeight := int64(100)
				err := store.UpdateLastIndexedBlockHeightWithRDbHandle(pgxConn.ToHandle(), newHeight)
				Expect(err).To(BeNil())

				currentHeight, err := store.GetLastIndexedBlockHeight()
				Expect(err).To(BeNil())
				Expect(currentHeight).To(Equal(primptr.Int64(int64(100))))
			})
		})
	})
})