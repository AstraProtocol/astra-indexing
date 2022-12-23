package account

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/golang-migrate/migrate/v4/source/github"

	cosmosapp_interface "github.com/AstraProtocol/astra-indexing/appinterface/cosmosapp"
	"github.com/AstraProtocol/astra-indexing/appinterface/projection/rdbprojectionbase"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	event_entity "github.com/AstraProtocol/astra-indexing/entity/event"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/AstraProtocol/astra-indexing/external/tmcosmosutils"
	"github.com/AstraProtocol/astra-indexing/external/utctime"
	"github.com/AstraProtocol/astra-indexing/infrastructure/pg/migrationhelper"
	"github.com/AstraProtocol/astra-indexing/projection/account/view"
	account_transaction_view "github.com/AstraProtocol/astra-indexing/projection/account_transaction/view"
	"github.com/AstraProtocol/astra-indexing/usecase/coin"
	event_usecase "github.com/AstraProtocol/astra-indexing/usecase/event"
)

// Account number, sequence number, balances are fetched from the latest state (regardless of current replaying height)
type Account struct {
	*rdbprojectionbase.Base

	rdbConn      rdb.Conn
	logger       applogger.Logger
	cosmosClient cosmosapp_interface.Client // cosmos light client deamon port : 1317 (default)

	migrationHelper migrationhelper.MigrationHelper
}

func NewAccount(
	logger applogger.Logger,
	rdbConn rdb.Conn,
	cosmosClient cosmosapp_interface.Client,
	migrationHelper migrationhelper.MigrationHelper,
) *Account {
	return &Account{
		rdbprojectionbase.NewRDbBase(
			rdbConn.ToHandle(),
			"Account",
		),

		rdbConn,
		logger,
		cosmosClient,

		migrationHelper,
	}
}

var (
	NewAccountsView              = view.NewAccountsView
	UpdateLastHandledEventHeight = (*Account).UpdateLastHandledEventHeight
)

func (_ *Account) GetEventsToListen() []string {
	return append([]string{
		// TODO: Genesis account
		event_usecase.ACCOUNT_TRANSFERRED,
		event_usecase.TRANSACTION_CREATED,
		event_usecase.TRANSACTION_FAILED,
	}, event_usecase.MSG_EVENTS...)
}

func (projection *Account) OnInit() error {
	if projection.migrationHelper != nil {
		projection.migrationHelper.Migrate()
	}
	return nil
}

func (projection *Account) HandleEvents(height int64, events []event_entity.Event) error {
	rdbTx, err := projection.rdbConn.Begin()
	if err != nil {
		return fmt.Errorf("error beginning transaction: %v", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = rdbTx.Rollback()
		}
	}()

	rdbTxHandle := rdbTx.ToHandle()

	accountsView := NewAccountsView(rdbTxHandle)
	accountGasUsedTotalView := view.NewAccountGasUsedTotal(rdbTxHandle)

	// Handle and insert a single copy of transaction data
	txs := make([]account_transaction_view.TransactionRow, 0)
	txMsgs := make(map[string][]event_usecase.MsgEvent)
	for _, event := range events {
		if transactionCreatedEvent, ok := event.(*event_usecase.TransactionCreated); ok {
			txs = append(txs, account_transaction_view.TransactionRow{
				BlockHeight:   height,
				BlockTime:     utctime.UTCTime{}, // placeholder
				Hash:          transactionCreatedEvent.TxHash,
				Index:         transactionCreatedEvent.Index,
				Success:       true,
				Code:          transactionCreatedEvent.Code,
				Log:           transactionCreatedEvent.Log,
				Fee:           transactionCreatedEvent.Fee,
				FeePayer:      transactionCreatedEvent.FeePayer,
				FeeGranter:    transactionCreatedEvent.FeeGranter,
				GasWanted:     transactionCreatedEvent.GasWanted,
				GasUsed:       transactionCreatedEvent.GasUsed,
				Memo:          transactionCreatedEvent.Memo,
				TimeoutHeight: transactionCreatedEvent.TimeoutHeight,
				Messages:      make([]account_transaction_view.TransactionRowMessage, 0),
			})
		} else if transactionFailedEvent, ok := event.(*event_usecase.TransactionFailed); ok {
			txs = append(txs, account_transaction_view.TransactionRow{
				BlockHeight:   height,
				BlockTime:     utctime.UTCTime{}, // placeholder
				Hash:          transactionFailedEvent.TxHash,
				Index:         transactionFailedEvent.Index,
				Success:       false,
				Code:          transactionFailedEvent.Code,
				Log:           transactionFailedEvent.Log,
				Fee:           transactionFailedEvent.Fee,
				FeePayer:      transactionFailedEvent.FeePayer,
				FeeGranter:    transactionFailedEvent.FeeGranter,
				GasWanted:     transactionFailedEvent.GasWanted,
				GasUsed:       transactionFailedEvent.GasUsed,
				Memo:          transactionFailedEvent.Memo,
				TimeoutHeight: transactionFailedEvent.TimeoutHeight,
				Messages:      make([]account_transaction_view.TransactionRowMessage, 0),
			})
		} else if msgEvent, ok := event.(event_usecase.MsgEvent); ok {
			if _, exist := txMsgs[msgEvent.TxHash()]; !exist {
				txMsgs[msgEvent.TxHash()] = make([]event_usecase.MsgEvent, 0)
			}
			txMsgs[msgEvent.TxHash()] = append(txMsgs[msgEvent.TxHash()], msgEvent)
		}
	}

	for _, tx := range txs {
		// Parse sender address from message event
		var msgEvent event_usecase.MsgEvent
		for _, msg := range txMsgs[tx.Hash] {
			msgEvent = msg
			break
		}
		senderAddress := projection.ParseSenderAddressFromMsgEvent(msgEvent)
		projection.logger.Infof("Sender address: %v", senderAddress)

		// Calculate account gas used total
		var address string
		if tmcosmosutils.IsValidCosmosAddress(senderAddress) {
			_, converted, _ := tmcosmosutils.DecodeAddressToHex(senderAddress)
			address = "0x" + hex.EncodeToString(converted)
		} else {
			return fmt.Errorf("error preparing total gas used of account: account is invalid")
		}

		if err := accountGasUsedTotalView.Increment(address, int64(tx.GasUsed)); err != nil {
			return fmt.Errorf("error incrementing total gas used of account: %w", err)
		}
	}

	for _, event := range events {
		if accountCreatedEvent, ok := event.(*event_usecase.AccountTransferred); ok {
			if handleErr := projection.handleAccountCreatedEvent(accountsView, accountCreatedEvent); handleErr != nil {
				return fmt.Errorf("error handling AccountCreatedEvent: %v", handleErr)
			}
		}
	}

	if err = UpdateLastHandledEventHeight(projection, rdbTxHandle, height); err != nil {
		return fmt.Errorf("error updating last handled event height: %v", err)
	}

	if err = rdbTx.Commit(); err != nil {
		return fmt.Errorf("error committing changes: %v", err)
	}
	committed = true

	return nil
}

func (projection *Account) handleAccountCreatedEvent(accountsView view.Accounts, event *event_usecase.AccountTransferred) error {

	recipienterr := projection.writeAccountInfo(accountsView, event.Recipient)
	if recipienterr != nil {
		return recipienterr
	}

	sendererr := projection.writeAccountInfo(accountsView, event.Sender)
	if sendererr != nil {
		return sendererr
	}

	return nil
}

func (projection *Account) getAccountInfo(address string) (*cosmosapp_interface.Account, error) {
	var accountInfo, accountInfoError = projection.cosmosClient.Account(address)
	if accountInfoError != nil {
		return nil, accountInfoError
	}

	return accountInfo, nil
}

func (projection *Account) getAccountBalances(targetAddress string) (coin.Coins, error) {
	var balanceInfo, balanceInfoError = projection.cosmosClient.Balances(targetAddress)
	if balanceInfoError != nil {
		return nil, balanceInfoError
	}

	return balanceInfo, nil
}

func (projection *Account) writeAccountInfo(accountsView view.Accounts, address string) error {
	accountInfo, err := projection.getAccountInfo(address)
	if err != nil {
		return err
	}

	accountType := accountInfo.Type
	var name *string
	if accountInfo.Type == cosmosapp_interface.ACCOUNT_MODULE {
		name = &accountInfo.MaybeModuleAccount.Name
	}
	var pubkey *string
	if accountInfo.MaybePubkey != nil {
		pubkey = &accountInfo.MaybePubkey.Key
	}
	accountNumber := accountInfo.AccountNumber
	sequenceNumber := accountInfo.Sequence

	balances, err := projection.getAccountBalances(address)
	if err != nil {
		return err
	}
	if err := accountsView.Upsert(&view.AccountRow{
		Type:           accountType,
		Address:        address,
		MaybeName:      name,
		MaybePubkey:    pubkey,
		AccountNumber:  accountNumber,
		SequenceNumber: sequenceNumber,
		Balance:        balances,
	}); err != nil {
		return err
	}

	return nil
}

func (projection *Account) ParseSenderAddressFromMsgEvent(msgEvent event_usecase.MsgEvent) string {
	// TODO: implement this
	msg := msgEvent.String()
	projection.logger.Infof("Message event: %v", msg)
	if strings.Contains(msg, "FromAddress") {
		rgx := regexp.MustCompile(`FromAddress:"([a-zA-Z0-9]+)"`)
		rs := rgx.FindStringSubmatch(msg)
		return rs[1]
	} else if strings.Contains(msg, "From") {
		rgx := regexp.MustCompile(`From:"(0x[a-zA-Z0-9]+)"`)
		rs := rgx.FindStringSubmatch(msg)
		return rs[1]
	} else if strings.Contains(msg, "Grantee") {
		rgx := regexp.MustCompile(`Grantee:"([a-zA-Z0-9]+)"`)
		rs := rgx.FindStringSubmatch(msg)
		return rs[1]
	} else if strings.Contains(msg, "DelegatorAddress") {
		rgx := regexp.MustCompile(`DelegatorAddress:"([a-zA-Z0-9]+)"`)
		rs := rgx.FindStringSubmatch(msg)
		return rs[1]
	}
	return ""
}
