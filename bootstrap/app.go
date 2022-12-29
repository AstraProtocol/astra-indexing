package bootstrap

import (
	"errors"
	"fmt"
	"time"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdbchainstatsstore"
	config "github.com/AstraProtocol/astra-indexing/bootstrap/config"
	projection_entity "github.com/AstraProtocol/astra-indexing/entity/projection"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"
	"github.com/AstraProtocol/astra-indexing/infrastructure/pg"
	"github.com/golang-migrate/migrate/v4"
	"gopkg.in/robfig/cron.v2"
)

type app struct {
	logger applogger.Logger
	config *config.Config

	rdbConn       rdb.Conn
	httpAPIServer *HTTPAPIServer
	indexService  *IndexService
}

func NewApp(logger applogger.Logger, config *config.Config) *app {
	rdbConn, err := SetupRDbConn(config, logger)
	if err != nil {
		logger.Panicf("error setting up RDb connection: %v", err)
	}

	if config.IndexService.Enable {
		ref := ""
		if config.IndexService.GithubAPI.MigrationRepoRef != "" {
			ref = "#" + config.IndexService.GithubAPI.MigrationRepoRef

			m, err := migrate.New(
				fmt.Sprintf(MIGRATION_GITHUB_TARGET, config.IndexService.GithubAPI.Username, config.IndexService.GithubAPI.Token, ref),
				migrationDBConnString(rdbConn),
			)
			if err != nil {
				logger.Panicf("failed to init migration: %v", err)
			}

			if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
				logger.Panicf("failed to run migration: %v", err)
			}
		}
	}

	return &app{
		logger:  logger,
		config:  config,
		rdbConn: rdbConn,
	}
}

const (
	MIGRATION_TABLE_NAME    = "schema_migrations"
	MIGRATION_GITHUB_TARGET = "github://%s:%s@AstraProtocol/astra-indexing/migrations%s"
)

func migrationDBConnString(conn rdb.Conn) string {
	connString := conn.(*pg.PgxConn).ConnString()
	if connString[len(connString)-1:] == "?" {
		return connString + "x-migrations-table=" + MIGRATION_TABLE_NAME
	} else {
		return connString + "&x-migrations-table=" + MIGRATION_TABLE_NAME
	}
}

func (a *app) GetRDbConn() rdb.Conn {
	return a.rdbConn
}

func (a *app) InitHTTPAPIServer(registry RouteRegistry) {
	if a.config.HTTPService.Enable {
		a.httpAPIServer = NewHTTPAPIServer(a.logger, a.config)
		a.httpAPIServer.RegisterRoutes(registry)
	}
}

func (a *app) InitIndexService(projections []projection_entity.Projection, cronJobs []projection_entity.CronJob) {
	if a.config.IndexService.Enable {
		a.indexService = NewIndexService(a.logger, a.rdbConn, a.config, projections, cronJobs)
	}
}

func (a *app) Run() {
	if a.httpAPIServer != nil {
		go func() {
			if runErr := a.httpAPIServer.Run(); runErr != nil {
				a.logger.Panicf("%v", runErr)
			}
		}()
	}

	if a.indexService != nil {
		go func() {
			if runErr := a.indexService.Run(); runErr != nil {
				a.logger.Panicf("%v", runErr)
			}
		}()
	}

	if a.config.Prometheus.Enable {
		go func() {
			if runErr := prometheus.Run(a.config.Prometheus.ExportPath, a.config.Prometheus.Port); runErr != nil {
				a.logger.Panicf("%v", runErr)
			}
		}()
	}

	select {}
}

func (a *app) RunCronJobsStats(rdbHandle *rdb.Handle) {
	if a.config.CronjobStats.Enable {
		rdbTransactionStatsStore := rdbchainstatsstore.NewRDbChainStatsStore(rdbHandle)
		s := cron.New()

		// At 59 seconds past the minute, at 59 minutes past every hour from 0 through 23
		// @every 0h0m5s
		s.AddFunc("59 59 0-23 * * *", func() {
			currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
			go rdbTransactionStatsStore.UpdateCountedTransactionsWithRDbHandle(currentDate)
		})

		s.AddFunc("59 59 0-23 * * *", func() {
			currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
			time.Sleep(2 * time.Second)
			go rdbTransactionStatsStore.UpdateTotalGasUsedWithRDbHandle(currentDate)
		})

		s.AddFunc("59 59 0-23 * * *", func() {
			currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
			time.Sleep(4 * time.Second)
			go rdbTransactionStatsStore.UpdateTotalFeeWithRDbHandle(currentDate)
		})

		s.AddFunc("59 59 0-23 * * *", func() {
			currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
			time.Sleep(6 * time.Second)
			go rdbTransactionStatsStore.UpdateTotalAddressesWithRDbHandle(currentDate)
		})

		s.AddFunc("59 59 0-23 * * *", func() {
			currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
			time.Sleep(8 * time.Second)
			go rdbTransactionStatsStore.UpdateActiveAddressesWithRDbHandle(currentDate)
		})

		s.Start()
	}
}
