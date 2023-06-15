package transaction

import (
	"encoding/hex"
	"fmt"
	"strconv"

	evmUtil "github.com/AstraProtocol/astra-indexing/internal/evm"

	"github.com/AstraProtocol/astra-indexing/appinterface/projection/rdbprojectionbase"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	event_entity "github.com/AstraProtocol/astra-indexing/entity/event"
	projection_entity "github.com/AstraProtocol/astra-indexing/entity/projection"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/AstraProtocol/astra-indexing/external/tmcosmosutils"
	"github.com/AstraProtocol/astra-indexing/external/utctime"
	"github.com/AstraProtocol/astra-indexing/infrastructure/pg/migrationhelper"
	transaction_view "github.com/AstraProtocol/astra-indexing/projection/transaction/view"
	event_usecase "github.com/AstraProtocol/astra-indexing/usecase/event"
)

var _ projection_entity.Projection = &Transaction{}

var (
	NewTransactions              = transaction_view.NewTransactionsView
	NewTransactionsTotal         = transaction_view.NewTransactionsTotalView
	UpdateLastHandledEventHeight = (*Transaction).UpdateLastHandledEventHeight
)

type Transaction struct {
	*rdbprojectionbase.Base

	rdbConn rdb.Conn
	logger  applogger.Logger

	migrationHelper migrationhelper.MigrationHelper
	evmUtil         evmUtil.EvmUtils
}

func NewTransaction(
	logger applogger.Logger,
	rdbConn rdb.Conn,
	migrationHelper migrationhelper.MigrationHelper,
	evmUtil evmUtil.EvmUtils,
) *Transaction {
	return &Transaction{
		rdbprojectionbase.NewRDbBase(
			rdbConn.ToHandle(),
			"Transaction",
		),

		rdbConn,
		logger,
		migrationHelper,
		evmUtil,
	}
}

func (*Transaction) GetEventsToListen() []string {
	return append([]string{
		event_usecase.BLOCK_CREATED,
		event_usecase.TRANSACTION_CREATED,
		event_usecase.TRANSACTION_FAILED,
	}, event_usecase.MSG_EVENTS...)
}

// const (
// 	MIGRATION_TABLE_NAME = "transaction_schema_migrations"
// 	MIGRATION_DIRECOTRY  = "projection/transaction/migrations"
// )

func (projection *Transaction) OnInit() error {
	if projection.migrationHelper != nil {
		projection.migrationHelper.Migrate()
	}

	return nil
}

func (projection *Transaction) HandleEvents(height int64, events []event_entity.Event) error {
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
	transactionsView := NewTransactions(rdbTxHandle)
	transactionsTotalView := NewTransactionsTotal(rdbTxHandle)

	var blockTime utctime.UTCTime
	var blockHash string
	txs := make([]transaction_view.TransactionRow, 0)
	txMsgs := make(map[string][]event_usecase.MsgEvent)
	txEvmTypes := make(map[string]string)
	txEvmHashes := make(map[string]string)
	for _, event := range events {
		if blockCreatedEvent, ok := event.(*event_usecase.BlockCreated); ok {
			blockTime = blockCreatedEvent.Block.Time
			blockHash = blockCreatedEvent.Block.Hash
		} else if transactionCreatedEvent, ok := event.(*event_usecase.TransactionCreated); ok {
			tx := transaction_view.TransactionRow{
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
				Messages:      make([]transaction_view.TransactionRowMessage, 0),
			}

			signers := make([]transaction_view.TransactionRowSigner, 0)
			for _, transactionCreatedEventSigner := range transactionCreatedEvent.Signers {
				transactionRowSigner := transaction_view.TransactionRowSigner{
					AccountSequence: transactionCreatedEventSigner.AccountSequence,
					Address:         transactionCreatedEventSigner.Address,
				}
				if transactionCreatedEventSigner.MaybeKeyInfo != nil {
					transactionRowSigner.MaybeKeyInfo = &transaction_view.TransactionRowSignerKeyInfo{
						Type:           transactionCreatedEventSigner.MaybeKeyInfo.Type,
						IsMultiSig:     transactionCreatedEventSigner.MaybeKeyInfo.IsMultiSig,
						Pubkeys:        transactionCreatedEventSigner.MaybeKeyInfo.Pubkeys,
						MaybeThreshold: transactionCreatedEventSigner.MaybeKeyInfo.MaybeThreshold,
					}
				}
				signers = append(signers, transactionRowSigner)
			}
			tx.Signers = signers
			txs = append(txs, tx)
		} else if transactionFailedEvent, ok := event.(*event_usecase.TransactionFailed); ok {
			tx := transaction_view.TransactionRow{
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
				Messages:      make([]transaction_view.TransactionRowMessage, 0),
			}

			signers := make([]transaction_view.TransactionRowSigner, 0)
			for _, transactionFailedEventSigner := range transactionFailedEvent.Signers {
				transactionRowSigner := transaction_view.TransactionRowSigner{
					AccountSequence: transactionFailedEventSigner.AccountSequence,
					Address:         transactionFailedEventSigner.Address,
				}
				if transactionFailedEventSigner.MaybeKeyInfo != nil {
					transactionRowSigner.MaybeKeyInfo = &transaction_view.TransactionRowSignerKeyInfo{
						Type:           transactionFailedEventSigner.MaybeKeyInfo.Type,
						IsMultiSig:     transactionFailedEventSigner.MaybeKeyInfo.IsMultiSig,
						Pubkeys:        transactionFailedEventSigner.MaybeKeyInfo.Pubkeys,
						MaybeThreshold: transactionFailedEventSigner.MaybeKeyInfo.MaybeThreshold,
					}
				}
				signers = append(signers, transactionRowSigner)
			}

			tx.Signers = signers
			txs = append(txs, tx)
		} else if msgEvent, ok := event.(event_usecase.MsgEvent); ok {
			if _, exist := txMsgs[msgEvent.TxHash()]; !exist {
				txMsgs[msgEvent.TxHash()] = make([]event_usecase.MsgEvent, 0)
			}
			txMsgs[msgEvent.TxHash()] = append(txMsgs[msgEvent.TxHash()], msgEvent)
		}
	}

	for _, event := range events {
		if txEvmEvent, ok := event.(*event_usecase.MsgEthereumTx); ok {
			evmType := projection.evmUtil.GetMethodNameFromData(txEvmEvent.Params.Data.Data)
			txEvmTypes[txEvmEvent.TxHash()] = evmType
			txEvmHashes[txEvmEvent.TxHash()] = txEvmEvent.Params.Hash
		}
	}

	if len(txs) == 0 {
		if err := projection.UpdateLastHandledEventHeight(rdbTxHandle, height); err != nil {
			return fmt.Errorf("error updating last handled event height: %v", err)
		}

		if err := rdbTx.Commit(); err != nil {
			return fmt.Errorf("error committing changes: %v", err)
		}
		committed = true
		return nil
	}

	for i, tx := range txs {
		txs[i].BlockTime = blockTime
		txs[i].BlockHash = blockHash

		for _, msg := range txMsgs[tx.Hash] {
			tmpMessage := transaction_view.TransactionRowMessage{
				Type:    msg.MsgType(),
				Content: msg,
			}

			if val, ok := txEvmTypes[tx.Hash]; ok {
				tmpMessage.EvmType = val
			}
			txs[i].Messages = append(txs[i].Messages, tmpMessage)
			txs[i].EvmHash = txEvmHashes[tx.Hash]
		}

		fromAddress := ""
		if len(txMsgs[tx.Hash]) > 0 {
			fromAddress = tmcosmosutils.ParseSenderAddressFromMsgEvent(txMsgs[tx.Hash][0])
		}
		if tmcosmosutils.IsValidCosmosAddress(fromAddress) {
			_, converted, _ := tmcosmosutils.DecodeAddressToHex(fromAddress)
			fromAddress = "0x" + hex.EncodeToString(converted)
		} else {
			if !evmUtil.IsHexAddress(fromAddress) {
				fromAddress = ""
			}
		}
		txs[i].FromAddress = fromAddress
	}
	if insertErr := transactionsView.InsertAll(txs); insertErr != nil {
		return fmt.Errorf("error inserting transaction into view: %v", insertErr)
	}

	totalTxs := int64(len(txs))
	if err := transactionsTotalView.Increment("-", totalTxs); err != nil {
		return fmt.Errorf("error incremnting total transactions: %w", err)
	}
	if err := transactionsTotalView.Set(strconv.FormatInt(height, 10), totalTxs); err != nil {
		return fmt.Errorf("error setting total blcok transactions: %w", err)
	}

	if err := UpdateLastHandledEventHeight(projection, rdbTxHandle, height); err != nil {
		return fmt.Errorf("error updating last handled event height: %v", err)
	}

	if err := rdbTx.Commit(); err != nil {
		return fmt.Errorf("error committing changes: %v", err)
	}
	committed = true
	return nil
}
