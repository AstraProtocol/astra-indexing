package account_transaction

import (
	"encoding/hex"
	"fmt"
	"strings"

	evmUtil "github.com/AstraProtocol/astra-indexing/internal/evm"

	"github.com/AstraProtocol/astra-indexing/appinterface/projection/rdbprojectionbase"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	event_entity "github.com/AstraProtocol/astra-indexing/entity/event"
	projection_entity "github.com/AstraProtocol/astra-indexing/entity/projection"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/AstraProtocol/astra-indexing/external/tmcosmosutils"
	"github.com/AstraProtocol/astra-indexing/external/utctime"
	"github.com/AstraProtocol/astra-indexing/infrastructure/pg/migrationhelper"
	"github.com/AstraProtocol/astra-indexing/projection/account_transaction/view"
	event_usecase "github.com/AstraProtocol/astra-indexing/usecase/event"
	"github.com/AstraProtocol/astra-indexing/usecase/model"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	_ projection_entity.Projection = &AccountTransaction{}
)

type AccountTransaction struct {
	*rdbprojectionbase.Base

	accountAddressPrefix string

	rdbConn rdb.Conn
	logger  applogger.Logger

	migrationHelper migrationhelper.MigrationHelper
	evmUtil         evmUtil.EvmUtils
}

func NewAccountTransaction(
	logger applogger.Logger,
	rdbConn rdb.Conn,
	accountAddressPrefix string,
	migrationHelper migrationhelper.MigrationHelper,
	evmUtil evmUtil.EvmUtils,
) *AccountTransaction {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("astra", "astrapub")
	return &AccountTransaction{
		rdbprojectionbase.NewRDbBase(
			rdbConn.ToHandle(),
			"AccountTransaction",
		),

		accountAddressPrefix,

		rdbConn,
		logger,

		migrationHelper,
		evmUtil,
	}
}

func (*AccountTransaction) GetEventsToListen() []string {
	return append([]string{
		event_usecase.BLOCK_CREATED,
		event_usecase.TRANSACTION_CREATED,
		event_usecase.TRANSACTION_FAILED,
	}, event_usecase.MSG_EVENTS...)
}

func (projection *AccountTransaction) OnInit() error {
	if projection.migrationHelper != nil {
		projection.migrationHelper.Migrate()
	}
	return nil
}

func (projection *AccountTransaction) HandleEvents(height int64, events []event_entity.Event) error {
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

	// TODO: Handle genesis transaction
	if height == int64(0) {
		if err := projection.UpdateLastHandledEventHeight(rdbTxHandle, height); err != nil {
			return fmt.Errorf("error updating last handled event height: %v", err)
		}

		if err := rdbTx.Commit(); err != nil {
			return fmt.Errorf("error committing changes: %v", err)
		}
		committed = true
		return nil
	}

	accountTransactionsView := view.NewAccountTransactions(rdbTxHandle)
	accountTransactionDataView := view.NewAccountTransactionData(rdbTxHandle)
	accountTransactionsTotalView := view.NewAccountTransactionsTotal(rdbTxHandle)
	accountGasUsedTotalView := view.NewAccountGasUsedTotal(rdbTxHandle)
	// accountFeesTotalView := view.NewAccountFeesTotal(rdbTxHandle)

	var blockTime utctime.UTCTime
	var blockHash string

	transactionInfos := make(map[string]*TransactionInfo)

	// Handle and insert a single copy of transaction data
	txs := make([]view.TransactionRow, 0)
	txMsgs := make(map[string][]event_usecase.MsgEvent)
	for _, event := range events {
		if blockCreatedEvent, ok := event.(*event_usecase.BlockCreated); ok {
			blockTime = blockCreatedEvent.Block.Time
			blockHash = blockCreatedEvent.Block.Hash
		} else if transactionCreatedEvent, ok := event.(*event_usecase.TransactionCreated); ok {
			txs = append(txs, view.TransactionRow{
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
				Messages:      make([]view.TransactionRowMessage, 0),
			})

			transactionInfos[transactionCreatedEvent.TxHash] = NewTransactionInfo(
				view.AccountTransactionBaseRow{
					Account:      "", // placeholder
					BlockHeight:  height,
					BlockHash:    "",                // placeholder
					BlockTime:    utctime.UTCTime{}, // placeholder
					Hash:         transactionCreatedEvent.TxHash,
					MessageTypes: []string{},
					Success:      true,
				},
			)
			senders := projection.ParseSenderAddresses(transactionCreatedEvent.Senders)
			for _, sender := range senders {
				transactionInfos[transactionCreatedEvent.TxHash].AddAccount(sender)
			}
		} else if transactionFailedEvent, ok := event.(*event_usecase.TransactionFailed); ok {
			row := view.TransactionRow{
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
				Messages:      make([]view.TransactionRowMessage, 0),
			}
			txs = append(txs, row)

			transactionInfos[transactionFailedEvent.TxHash] = NewTransactionInfo(
				view.AccountTransactionBaseRow{
					Account:      "", // placeholder
					BlockHeight:  height,
					BlockHash:    "",                // placeholder
					BlockTime:    utctime.UTCTime{}, // placeholder
					Hash:         transactionFailedEvent.TxHash,
					MessageTypes: []string{},
					Success:      false,
				},
			)
			senders := projection.ParseSenderAddresses(transactionFailedEvent.Senders)
			for _, sender := range senders {
				transactionInfos[transactionFailedEvent.TxHash].AddAccount(sender)
			}
		} else if msgEvent, ok := event.(event_usecase.MsgEvent); ok {
			if _, exist := txMsgs[msgEvent.TxHash()]; !exist {
				txMsgs[msgEvent.TxHash()] = make([]event_usecase.MsgEvent, 0)
			}
			txMsgs[msgEvent.TxHash()] = append(txMsgs[msgEvent.TxHash()], msgEvent)

			transactionInfos[msgEvent.TxHash()].AddMessageTypes(msgEvent.MsgType())
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

	txEvmType := make(map[string]string)

	// Handle transaction messages
	for _, event := range events {
		if typedEvent, ok := event.(*event_usecase.MsgSend); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.FromAddress)
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.ToAddress)

		} else if typedEvent, ok := event.(*event_usecase.MsgMultiSend); ok {
			for _, input := range typedEvent.Inputs {
				transactionInfos[typedEvent.TxHash()].AddAccount(input.Address)
			}
			for _, output := range typedEvent.Outputs {
				transactionInfos[typedEvent.TxHash()].AddAccount(output.Address)
			}

		} else if typedEvent, ok := event.(*event_usecase.MsgSetWithdrawAddress); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.DelegatorAddress)
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.WithdrawAddress)

		} else if typedEvent, ok := event.(*event_usecase.MsgWithdrawDelegatorReward); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.DelegatorAddress)

		} else if typedEvent, ok := event.(*event_usecase.MsgWithdrawValidatorCommission); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.RecipientAddress)

		} else if typedEvent, ok := event.(*event_usecase.MsgFundCommunityPool); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Depositor)

		} else if typedEvent, ok := event.(*event_usecase.MsgFundCommunityPool); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Depositor)

		} else if typedEvent, ok := event.(*event_usecase.MsgSubmitTextProposal); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.ProposerAddress)

		} else if typedEvent, ok := event.(*event_usecase.MsgSubmitParamChangeProposal); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.ProposerAddress)

		} else if typedEvent, ok := event.(*event_usecase.MsgSubmitCommunityPoolSpendProposal); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.ProposerAddress)

		} else if typedEvent, ok := event.(*event_usecase.MsgSubmitSoftwareUpgradeProposal); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.ProposerAddress)

		} else if typedEvent, ok := event.(*event_usecase.MsgSubmitCancelSoftwareUpgradeProposal); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.ProposerAddress)

		} else if typedEvent, ok := event.(*event_usecase.MsgSubmitUnknownProposal); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.ProposerAddress)

		} else if typedEvent, ok := event.(*event_usecase.MsgDeposit); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Depositor)

		} else if typedEvent, ok := event.(*event_usecase.MsgVote); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Voter)

		} else if typedEvent, ok := event.(*event_usecase.MsgCreateValidator); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.DelegatorAddress)

		} else if typedEvent, ok := event.(*event_usecase.MsgEditValidator); ok {
			accountAddress := tmcosmosutils.MustAccountAddressFromValidatorAddress(
				projection.accountAddressPrefix,
				typedEvent.ValidatorAddress,
			)
			transactionInfos[typedEvent.TxHash()].AddAccount(accountAddress)

		} else if typedEvent, ok := event.(*event_usecase.MsgDelegate); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.DelegatorAddress)

		} else if typedEvent, ok := event.(*event_usecase.MsgUndelegate); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.DelegatorAddress)

		} else if typedEvent, ok := event.(*event_usecase.MsgBeginRedelegate); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.DelegatorAddress)

		} else if typedEvent, ok := event.(*event_usecase.MsgUnjail); ok {
			accountAddress := tmcosmosutils.MustAccountAddressFromValidatorAddress(
				projection.accountAddressPrefix,
				typedEvent.ValidatorAddr,
			)
			transactionInfos[typedEvent.TxHash()].AddAccount(accountAddress)

		} else if typedEvent, ok := event.(*event_usecase.MsgIBCCreateClient); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Signer)

		} else if typedEvent, ok := event.(*event_usecase.MsgIBCUpdateClient); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Signer)

		} else if typedEvent, ok := event.(*event_usecase.MsgIBCConnectionOpenInit); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Signer)

		} else if typedEvent, ok := event.(*event_usecase.MsgIBCConnectionOpenAck); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Signer)

		} else if typedEvent, ok := event.(*event_usecase.MsgIBCConnectionOpenTry); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Signer)

		} else if typedEvent, ok := event.(*event_usecase.MsgIBCConnectionOpenConfirm); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Signer)

		} else if typedEvent, ok := event.(*event_usecase.MsgIBCChannelOpenInit); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Signer)

		} else if typedEvent, ok := event.(*event_usecase.MsgIBCChannelOpenAck); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Signer)

		} else if typedEvent, ok := event.(*event_usecase.MsgIBCChannelOpenTry); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Signer)

		} else if typedEvent, ok := event.(*event_usecase.MsgIBCChannelOpenConfirm); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Signer)

		} else if typedEvent, ok := event.(*event_usecase.MsgIBCAcknowledgement); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Signer)

			if typedEvent.Params.MaybeFungibleTokenPacketData != nil {
				transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.MaybeFungibleTokenPacketData.Sender)
			}

		} else if typedEvent, ok := event.(*event_usecase.MsgIBCRecvPacket); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Signer)

			if typedEvent.Params.MaybeFungibleTokenPacketData != nil {
				transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.MaybeFungibleTokenPacketData.Receiver)
			}

		} else if typedEvent, ok := event.(*event_usecase.MsgIBCTransferTransfer); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Sender)

		} else if typedEvent, ok := event.(*event_usecase.MsgIBCTimeout); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Signer)

			if typedEvent.Params.MaybeMsgTransfer != nil {
				transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.MaybeMsgTransfer.RefundReceiver)
			}

		} else if typedEvent, ok := event.(*event_usecase.MsgIBCTimeoutOnClose); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Signer)

			if typedEvent.Params.MaybeMsgTransfer != nil {
				transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.MaybeMsgTransfer.RefundReceiver)
			}

		} else if typedEvent, ok := event.(*event_usecase.MsgIBCChannelCloseInit); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Signer)

		} else if typedEvent, ok := event.(*event_usecase.MsgIBCChannelCloseConfirm); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Signer)

		} else if typedEvent, ok := event.(*event_usecase.MsgGrant); ok {

			if typedEvent.Params.MaybeSendGrant != nil {
				transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.MaybeSendGrant.Granter)
				transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.MaybeSendGrant.Grantee)

			} else if typedEvent.Params.MaybeStakeGrant != nil {
				transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.MaybeStakeGrant.Granter)
				transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.MaybeStakeGrant.Grantee)

			} else if typedEvent.Params.MaybeGenericGrant != nil {
				transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.MaybeGenericGrant.Granter)
				transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.MaybeGenericGrant.Grantee)

			}

		} else if typedEvent, ok := event.(*event_usecase.MsgRevoke); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Granter)
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Grantee)

		} else if typedEvent, ok := event.(*event_usecase.MsgExec); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Grantee)

		} else if typedEvent, ok := event.(*event_usecase.MsgGrantAllowance); ok {

			if typedEvent.Params.MaybeBasicAllowance != nil {
				transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.MaybeBasicAllowance.Granter)
				transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.MaybeBasicAllowance.Grantee)

			} else if typedEvent.Params.MaybePeriodicAllowance != nil {
				transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.MaybePeriodicAllowance.Granter)
				transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.MaybePeriodicAllowance.Grantee)

			} else if typedEvent.Params.MaybeAllowedMsgAllowance != nil {
				transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.MaybeAllowedMsgAllowance.Granter)
				transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.MaybeAllowedMsgAllowance.Grantee)

			}

		} else if typedEvent, ok := event.(*event_usecase.MsgRevokeAllowance); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Granter)
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.Grantee)
		} else if typedEvent, ok := event.(*event_usecase.MsgClawback); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.FunderAddress)
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.AccountAddress)
		} else if typedEvent, ok := event.(*event_usecase.MsgCreateVestingAccount); ok {
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.FromAddress)
			transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.ToAddress)
		} else if typedEvent, ok := event.(*event_usecase.MsgEthereumTx); ok {
			if isHexString(typedEvent.Params.From) {
				astraAddr, _ := sdk.AccAddressFromHex(typedEvent.Params.From[2:])
				transactionInfos[typedEvent.TxHash()].AddAccount(astraAddr.String())
			} else if len(typedEvent.Params.From) > 2 {
				transactionInfos[typedEvent.TxHash()].AddAccount(typedEvent.Params.From)
			}
			if isHexString(typedEvent.Params.Data.To) {
				astraAddr, _ := sdk.AccAddressFromHex(typedEvent.Params.Data.To[2:])
				transactionInfos[typedEvent.TxHash()].AddAccount(astraAddr.String())
			}
			evmType := projection.evmUtil.GetSignatureFromData(typedEvent.Params.Data.Data)
			txEvmType[typedEvent.TxHash()] = evmType
		}
	}

	for i, tx := range txs {
		txs[i].BlockTime = blockTime
		txs[i].BlockHash = blockHash

		transactionInfos[tx.Hash].FillBlockInfo(blockHash, blockTime)

		for _, msg := range txMsgs[tx.Hash] {
			tmpMessage := view.TransactionRowMessage{
				Type:    msg.MsgType(),
				Content: msg,
			}
			if val, ok := txEvmType[tx.Hash]; ok {
				tmpMessage.EvmType = val
			}
			txs[i].Messages = append(txs[i].Messages, tmpMessage)
		}

		var msgEvent event_usecase.MsgEvent
		senderAddress := ""
		if len(txMsgs[tx.Hash]) > 0 {
			msgEvent = txMsgs[tx.Hash][0]
			senderAddress = tmcosmosutils.ParseSenderAddressFromMsgEvent(msgEvent)
		}

		// Convert fees unit from aastra to microAstra
		/*
			divisor := big.NewFloat(0).SetInt(big.NewInt(0).Exp(big.NewInt(10), big.NewInt(12), nil))
			microAstraFees := big.NewFloat(0).SetInt(tx.Fee.AmountOf("aastra").BigInt())
			microAstraFees = microAstraFees.Quo(microAstraFees, divisor)
			fees, _ := microAstraFees.Float64()
		*/

		// Calculate account gas used and account fees total
		if tmcosmosutils.IsValidCosmosAddress(senderAddress) {
			_, converted, _ := tmcosmosutils.DecodeAddressToHex(senderAddress)
			address := "0x" + hex.EncodeToString(converted)

			if err := accountGasUsedTotalView.Increment(address, int64(tx.GasUsed)); err != nil {
				return fmt.Errorf("error incrementing total gas used of account: %w", err)
			}

			/*
				if err := accountFeesTotalView.Increment(address, int64(math.Round(fees))); err != nil {
					return fmt.Errorf("error incrementing total fees of account: %w", err)
				}
			*/
		} else {
			if evmUtil.IsHexAddress(senderAddress) {
				if err := accountGasUsedTotalView.Increment(senderAddress, int64(tx.GasUsed)); err != nil {
					return fmt.Errorf("error incrementing total gas used of account: %w", err)
				}

				/*
					if err := accountFeesTotalView.Increment(senderAddress, int64(math.Round(fees))); err != nil {
						return fmt.Errorf("error incrementing total fees of account: %w", err)
					}
				*/
			} else {
				if msgEvent == nil {
					projection.logger.Debug("message event is empty")
				} else {
					projection.logger.Debugf("error message event: %v", msgEvent.String())
				}
				projection.logger.Debugf("error preparing total gas used and total fees of account: %v", senderAddress)
			}
		}
	}
	if insertErr := accountTransactionDataView.InsertAll(txs); insertErr != nil {
		return fmt.Errorf("error inserting account transaction data into view: %v", insertErr)
	}

	accountTransactionRows := make([]view.AccountTransactionBaseRow, 0)
	for _, info := range transactionInfos {
		accountTransactionRows = append(accountTransactionRows, info.ToRows()...)
	}

	for _, tx := range txs {
		txInfo := transactionInfos[tx.Hash]
		rows := txInfo.ToRows()

		for _, row := range rows {

			// Calculate account transaction total
			if err := accountTransactionsTotalView.Increment(fmt.Sprintf("%s:-", row.Account), 1); err != nil {
				return fmt.Errorf("error incrementing total account transaction of account: %w", err)
			}
			for _, messageType := range row.MessageTypes {
				if err := accountTransactionsTotalView.Increment(
					fmt.Sprintf("%s:%s", row.Account, messageType), 1,
				); err != nil {
					return fmt.Errorf("error incrementing total account transaction message type of account: %w", err)
				}
			}

			// Calculate account/memo transaction total
			if tx.Memo != "" {
				accountWithMemo := row.Account + "/" + tx.Memo

				if err := accountTransactionsTotalView.Increment(fmt.Sprintf("%s:-", accountWithMemo), 1); err != nil {
					return fmt.Errorf("error incrementing total account transaction of account: %w", err)
				}
				for _, messageType := range row.MessageTypes {
					if err := accountTransactionsTotalView.Increment(
						fmt.Sprintf("%s:%s", accountWithMemo, messageType), 1,
					); err != nil {
						return fmt.Errorf("error incrementing total account transaction message type of account: %w", err)
					}
				}
			}

		}
	}

	if err := accountTransactionsView.InsertAll(accountTransactionRows); err != nil {
		return fmt.Errorf("error inserting account message: %w", err)
	}

	if err := projection.UpdateLastHandledEventHeight(rdbTxHandle, height); err != nil {
		return fmt.Errorf("error updating last handled event height: %v", err)
	}

	if err := rdbTx.Commit(); err != nil {
		return fmt.Errorf("error committing changes: %v", err)
	}
	committed = true
	return nil
}

func isHexString(s string) bool {
	if len(s) < 3 {
		return false
	}
	if !strings.HasPrefix(s, "0x") {
		return false
	}
	_, err := hex.DecodeString(s[2:])
	return err == nil
}

func (projection *AccountTransaction) ParseSenderAddresses(senders []model.TransactionSigner) []string {
	addresses := make([]string, 0, len(senders))
	for _, sender := range senders {
		addresses = append(addresses, sender.Address)
	}
	return addresses
}

type TransactionInfo struct {
	row              view.AccountTransactionBaseRow
	involvedAccounts map[string]bool // Set data structure
	messageTypes     map[string]bool // Set data structure
}

func NewTransactionInfo(row view.AccountTransactionBaseRow) *TransactionInfo {
	return &TransactionInfo{
		row: row,

		involvedAccounts: make(map[string]bool),
		messageTypes:     make(map[string]bool),
	}
}

func (info *TransactionInfo) IsAccountExist(address string) bool {
	return info.involvedAccounts[address]
}

func (info *TransactionInfo) AddAccount(address string) {
	info.involvedAccounts[address] = true
}

func (info *TransactionInfo) AddMessageTypes(messageTypes string) {
	info.messageTypes[messageTypes] = true
}

func (info *TransactionInfo) FillBlockInfo(blockHash string, blockTime utctime.UTCTime) {
	info.row.BlockHash = blockHash
	info.row.BlockTime = blockTime
}

func (info *TransactionInfo) ToRows() []view.AccountTransactionBaseRow {
	info.FillMessageTypes()

	rows := make([]view.AccountTransactionBaseRow, 0)
	for account := range info.involvedAccounts {
		clonedRow := info.row
		clonedRow.Account = account
		rows = append(rows, clonedRow)
	}

	return rows
}

func (info *TransactionInfo) FillMessageTypes() {
	info.row.MessageTypes = make([]string, 0)
	for messageType := range info.messageTypes {
		info.row.MessageTypes = append(info.row.MessageTypes, messageType)
	}
}
