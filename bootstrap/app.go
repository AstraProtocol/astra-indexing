package bootstrap

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdbchainstatsstore"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdbreportdashboard"
	"github.com/AstraProtocol/astra-indexing/bootstrap/config"
	projection_entity "github.com/AstraProtocol/astra-indexing/entity/projection"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	worker_consumer "github.com/AstraProtocol/astra-indexing/infrastructure/kafka/consumer/worker"
	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"
	"github.com/AstraProtocol/astra-indexing/infrastructure/pg"
	"github.com/AstraProtocol/astra-indexing/internal/evm"
	"github.com/golang-migrate/migrate/v4"
	"gopkg.in/robfig/cron.v2"
)

type app struct {
	logger  applogger.Logger
	config  *config.Config
	evmUtil evm.EvmUtils

	rdbConn       rdb.Conn
	httpAPIServer *HTTPAPIServer
	indexService  *IndexService
}

func NewApp(logger applogger.Logger, config *config.Config, evmUtil evm.EvmUtils) *app {
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
		evmUtil: evmUtil,
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

	if a.config.KafkaService.EnableConsumer {
		sigchan := make(chan os.Signal, 1)
		signal.Notify(sigchan, os.Interrupt)

		go func() {
			runErr := worker_consumer.RunInternalTxsConsumer(a.rdbConn.ToHandle(), a.config, a.logger, a.evmUtil, sigchan)
			if runErr != nil {
				a.logger.Errorf("%v", runErr)
			}
		}()
		go func() {
			runErr := worker_consumer.RunEvmTxsConsumer(a.rdbConn.ToHandle(), a.config, a.logger, sigchan)
			if runErr != nil {
				a.logger.Errorf("%v", runErr)
			}
		}()
		go func() {
			runErr := worker_consumer.RunTokenTransfersConsumer(a.rdbConn.ToHandle(), a.config, a.logger, sigchan)
			if runErr != nil {
				a.logger.Errorf("%v", runErr)
			}
		}()
	}
}

func (a *app) RunCronJobsStats(rdbHandle *rdb.Handle) {
	if a.config.CronjobStats.Enable {
		rdbChainStatsStore := rdbchainstatsstore.NewRDbChainStatsStore(rdbHandle)
		s := cron.New()

		delayTime := 60
		retry := 5

		var i int

		// At 59 seconds past the minute, at 59 minutes past every hour from 0 through 23
		// @every 0h0m5s
		// 59 59 0-23 * * *
		s.AddFunc("59 59 0-23 * * *", func() {
			currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
			i = 0
			var err error
			for i < retry {
				err = rdbChainStatsStore.UpdateCountedTransactionsWithRDbHandle(currentDate)
				if err == nil {
					break
				}
				fmt.Printf("failed to run UpdateCountedTransactionsWithRDbHandle cronjob: %v", err)
				time.Sleep(time.Duration(delayTime) * time.Second)
				i += 1
			}
		})

		s.AddFunc("59 59 0-23 * * *", func() {
			currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
			i = 0
			var err error
			time.Sleep(2 * time.Second)
			for i < retry {
				err = rdbChainStatsStore.UpdateTotalGasUsedWithRDbHandle(currentDate)
				if err == nil {
					break
				}
				fmt.Printf("failed to run UpdateTotalGasUsedWithRDbHandle cronjob: %v", err)
				time.Sleep(time.Duration(delayTime) * time.Second)
				i += 1
			}
		})

		s.AddFunc("59 59 0-23 * * *", func() {
			currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
			i = 0
			var err error
			time.Sleep(4 * time.Second)
			for i < retry {
				err = rdbChainStatsStore.UpdateTotalAddressesWithRDbHandle(currentDate)
				if err == nil {
					break
				}
				fmt.Printf("failed to run UpdateTotalAddressesWithRDbHandle cronjob: %v", err)
				time.Sleep(time.Duration(delayTime) * time.Second)
				i += 1
			}
		})

		s.AddFunc("59 59 0-23 * * *", func() {
			currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
			i = 0
			var err error
			time.Sleep(6 * time.Second)
			for i < retry {
				err = rdbChainStatsStore.UpdateActiveAddressesWithRDbHandle(currentDate)
				if err == nil {
					break
				}
				fmt.Printf("failed to run UpdateActiveAddressesWithRDbHandle cronjob: %v", err)
				time.Sleep(time.Duration(delayTime) * time.Second)
				i += 1
			}
		})

		s.AddFunc("59 59 0-23 * * *", func() {
			currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
			i = 0
			var err error
			time.Sleep(8 * time.Second)
			for i < retry {
				err = rdbChainStatsStore.UpdateTotalFeeWithRDbHandle(currentDate, a.config)
				if err == nil {
					break
				}
				fmt.Printf("failed to run UpdateTotalFeeWithRDbHandle cronjob: %v", err)
				time.Sleep(time.Duration(delayTime) * time.Second)
				i += 1
			}
		})

		s.Start()
	}
}

func (a *app) RunCronJobsReportDashboard(rdbHandle *rdb.Handle) {
	if a.config.CronjobReportDashboard.Enable {
		rdbReportDashboard := rdbreportdashboard.NewRDbReportDashboard(rdbHandle)
		s := cron.New()

		delayTime := 60
		retry := 5

		var i int

		// At 59 seconds past the minute, at 59 minutes past every hour from 0 through 23
		// @every 0h0m5s
		// 59 59 0-23 * * *
		s.AddFunc("59 59 0-23 * * *", func() {
			currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
			nextDate := time.Now().Truncate(24 * time.Hour).Add(24 * time.Hour).UnixNano()
			i = 0
			var err error
			time.Sleep(1 * time.Second)
			for i < retry {
				err = rdbReportDashboard.UpdateTotalAstraOnchainRewardsWithRDbHandle(currentDate, nextDate)
				if err == nil {
					break
				}
				fmt.Printf("failed to run UpdateTotalAstraOnchainRewardsWithRDbHandle cronjob: %v", err)
				time.Sleep(time.Duration(delayTime) * time.Second)
				i += 1
			}
		})

		s.AddFunc("59 59 0-23 * * *", func() {
			currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
			nextDate := time.Now().Truncate(24 * time.Hour).Add(24 * time.Hour).UnixNano()
			tikiAddress := a.config.CronjobReportDashboard.TikiAddress
			i = 0
			var err error
			time.Sleep(3 * time.Second)
			for i < retry {
				err = rdbReportDashboard.UpdateTotalAstraWithdrawnFromTikiWithRDbHandle(currentDate, nextDate, tikiAddress)
				if err == nil {
					break
				}
				fmt.Printf("failed to run UpdateTotalAstraWithdrawnFromTikiWithRDbHandle cronjob: %v", err)
				time.Sleep(time.Duration(delayTime) * time.Second)
				i += 1
			}
		})

		s.AddFunc("59 59 0-23 * * *", func() {
			currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
			nextDate := time.Now().Truncate(24 * time.Hour).Add(24 * time.Hour).UnixNano()
			i = 0
			var err error
			time.Sleep(5 * time.Second)
			for i < retry {
				err = rdbReportDashboard.UpdateTotalAstraOfRedeemedCouponsWithRDbHandle(currentDate, nextDate)
				if err == nil {
					break
				}
				fmt.Printf("failed to run UpdateTotalAstraOfRedeemedCouponsWithRDbHandle cronjob: %v", err)
				time.Sleep(time.Duration(delayTime) * time.Second)
				i += 1
			}
		})

		s.AddFunc("59 59 0-23 * * *", func() {
			currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
			nextDate := time.Now().Truncate(24 * time.Hour).Add(24 * time.Hour).UnixNano()
			i = 0
			var err error
			time.Sleep(7 * time.Second)
			for i < retry {
				err = rdbReportDashboard.UpdateTotalTxsOfRedeemedCouponsWithRDbHandle(currentDate, nextDate)
				if err == nil {
					break
				}
				fmt.Printf("failed to run UpdateTotalTxsOfRedeemedCouponsWithRDbHandle cronjob: %v", err)
				time.Sleep(time.Duration(delayTime) * time.Second)
				i += 1
			}
		})

		s.AddFunc("59 59 0-23 * * *", func() {
			currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
			nextDate := time.Now().Truncate(24 * time.Hour).Add(24 * time.Hour).UnixNano()
			i = 0
			var err error
			time.Sleep(9 * time.Second)
			for i < retry {
				err = rdbReportDashboard.UpdateTotalAddressesOfRedeemedCouponsWithRDbHandle(currentDate, nextDate)
				if err == nil {
					break
				}
				fmt.Printf("failed to run UpdateTotalAddressesOfRedeemedCouponsWithRDbHandle cronjob: %v", err)
				time.Sleep(time.Duration(delayTime) * time.Second)
				i += 1
			}
		})

		s.AddFunc("59 59 0-23 * * *", func() {
			currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
			nextDate := time.Now().Truncate(24 * time.Hour).Add(24 * time.Hour).UnixNano()
			i = 0
			var err error
			time.Sleep(11 * time.Second)
			for i < retry {
				err = rdbReportDashboard.UpdateTotalAstraStakedWithRDbHandle(currentDate, nextDate)
				if err == nil {
					break
				}
				fmt.Printf("failed to run UpdateTotalAstraStakedWithRDbHandle cronjob: %v", err)
				time.Sleep(time.Duration(delayTime) * time.Second)
				i += 1
			}
		})

		s.AddFunc("59 59 0-23 * * *", func() {
			currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
			nextDate := time.Now().Truncate(24 * time.Hour).Add(24 * time.Hour).UnixNano()
			i = 0
			var err error
			time.Sleep(13 * time.Second)
			for i < retry {
				err = rdbReportDashboard.UpdateTotalStakingTxsWithRDbHandle(currentDate, nextDate)
				if err == nil {
					break
				}
				fmt.Printf("failed to run UpdateTotalStakingTxsWithRDbHandle cronjob: %v", err)
				time.Sleep(time.Duration(delayTime) * time.Second)
				i += 1
			}
		})

		s.AddFunc("59 59 0-23 * * *", func() {
			currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
			nextDate := time.Now().Truncate(24 * time.Hour).Add(24 * time.Hour).UnixNano()
			i = 0
			var err error
			time.Sleep(15 * time.Second)
			for i < retry {
				err = rdbReportDashboard.UpdateTotalStakingAddressesWithRDbHandle(currentDate, nextDate)
				if err == nil {
					break
				}
				fmt.Printf("failed to run UpdateTotalStakingAddressesWithRDbHandle cronjob: %v", err)
				time.Sleep(time.Duration(delayTime) * time.Second)
				i += 1
			}
		})

		s.AddFunc("59 59 0-23 * * *", func() {
			currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
			prevDate := time.Now().Truncate(24 * time.Hour).Add(-24 * time.Hour).UnixNano()
			i = 0
			var err error
			time.Sleep(17 * time.Second)
			for i < retry {
				err = rdbReportDashboard.UpdateTotalNewAddressesWithRDbHandle(currentDate, prevDate)
				if err == nil {
					break
				}
				fmt.Printf("failed to run UpdateTotalNewAddressesWithRDbHandle cronjob: %v", err)
				time.Sleep(time.Duration(delayTime) * time.Second)
				i += 1
			}
		})

		s.Start()
	}
}
