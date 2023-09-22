package view

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/AstraProtocol/astra-indexing/external/json"
	"github.com/AstraProtocol/astra-indexing/usecase/coin"

	sq "github.com/Masterminds/squirrel"

	pagination_interface "github.com/AstraProtocol/astra-indexing/appinterface/pagination"
	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"

	jsoniter "github.com/json-iterator/go"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/external/tmcosmosutils"
	"github.com/AstraProtocol/astra-indexing/external/utctime"
)

const ALL = "all"
const SEND = "send"
const RECEIVE = "receive"
const REWARD = "reward"
const SAVING = "saving"
const EXCHANGE_COUPON = "exchange_coupon"

// BlockTransactions projection view implemented by relational database
type AccountTransactions struct {
	rdb *rdb.Handle
}

func NewAccountTransactions(handle *rdb.Handle) *AccountTransactions {
	return &AccountTransactions{
		handle,
	}
}

func (accountMessagesView *AccountTransactions) InsertAll(
	rows []AccountTransactionBaseRow,
) error {
	pendingRowCount := 0
	var stmtBuilder sq.InsertBuilder

	rowCount := len(rows)
	for i, row := range rows {
		if pendingRowCount == 0 {
			stmtBuilder = accountMessagesView.rdb.StmtBuilder.Insert(
				"view_account_transactions",
			).Columns(
				"block_height",
				"block_hash",
				"block_time",
				"account",
				"transaction_hash",
				"success",
				"message_types",
				"from_address",
				"to_address",
				"is_internal_tx",
				"tx_index",
			)
		}

		blockTime := accountMessagesView.rdb.Tton(&row.BlockTime)
		stmtBuilder = stmtBuilder.Values(
			row.BlockHeight,
			row.BlockHash,
			blockTime,
			row.Account,
			row.Hash,
			row.Success,
			json.MustMarshalToString(row.MessageTypes),
			row.FromAddress,
			row.ToAddress,
			row.IsInternalTx,
			row.TxIndex,
		)
		pendingRowCount += 1

		// Postgres has a limit of 65536 parameters.
		if pendingRowCount == 500 || i+1 == rowCount {
			sql, sqlArgs, err := stmtBuilder.ToSql()
			if err != nil {
				return fmt.Errorf("error building account transaction id insertion sql: %v: %w", err, rdb.ErrBuildSQLStmt)
			}

			result, err := accountMessagesView.rdb.Exec(sql, sqlArgs...)
			if err != nil {
				return fmt.Errorf("error inserting account transactions into the table: %v: %w", err, rdb.ErrWrite)
			}
			if result.RowsAffected() != int64(pendingRowCount) {
				return fmt.Errorf(
					"error inserting account transactions into the table: mismatch rows inserted: %w", rdb.ErrWrite,
				)
			}
			pendingRowCount = 0
		}
	}

	return nil
}

func (accountMessagesView *AccountTransactions) List(
	filter AccountTransactionsListFilter,
	order AccountTransactionsListOrder,
	pagination *pagination_interface.Pagination,
) ([]AccountTransactionReadRow, *pagination_interface.Result, error) {
	stmtBuilder := accountMessagesView.rdb.StmtBuilder.Select(
		"DISTINCT ON (view_account_transactions.id) view_account_transactions.id",
		"view_account_transactions.account",
		"view_account_transactions.block_height",
		"view_account_transactions.block_hash",
		"view_account_transactions.block_time",
		"view_account_transactions.transaction_hash",
		"view_account_transaction_data.success",
		"view_account_transaction_data.code",
		"view_account_transaction_data.log",
		"view_account_transaction_data.fee",
		"view_account_transaction_data.fee_payer",
		"view_account_transaction_data.fee_granter",
		"view_account_transaction_data.gas_wanted",
		"view_account_transaction_data.gas_used",
		"view_account_transaction_data.memo",
		"view_account_transaction_data.timeout_height",
		"view_account_transactions.message_types",
		"view_account_transaction_data.messages",
	).From(
		"view_account_transactions",
	).InnerJoin(
		"view_account_transaction_data ON view_account_transactions.block_height = view_account_transaction_data.block_height AND view_account_transactions.transaction_hash = view_account_transaction_data.hash",
	)

	if filter.TxType == "" {
		//include internal txs and token transfers
		if filter.IncludingInternalTx == "true" {
			stmtBuilder = stmtBuilder.Where(
				"(view_account_transactions.is_internal_tx = false AND view_account_transactions.account = ?) OR "+
					"(view_account_transactions.account = ? AND view_account_transactions.is_internal_tx = true AND "+
					"(view_account_transactions.from_address = view_account_transaction_data.from_address AND view_account_transactions.to_address = view_account_transaction_data.to_address))",
				filter.Account,
				filter.Account,
			)
		} else {
			if filter.Memo == "" {
				stmtBuilder = stmtBuilder.Where(
					"view_account_transactions.is_internal_tx = ? AND view_account_transactions.account = ?",
					false,
					filter.Account,
				)
			}

			if filter.Memo != "" {
				stmtBuilder = stmtBuilder.Where(
					"view_account_transactions.is_internal_tx = ? AND view_account_transactions.account = ? AND view_account_transaction_data.memo = ?",
					false,
					filter.Account,
					filter.Memo,
				)
			}
		}
	} else {
		// txs filter
		addressHash := strings.ToLower(filter.Account)
		if tmcosmosutils.IsValidCosmosAddress(filter.Account) {
			_, converted, _ := tmcosmosutils.DecodeAddressToHex(filter.Account)
			addressHash = strings.ToLower("0x" + hex.EncodeToString(converted))
		}

		// date time filter
		currentDate := time.Now().Truncate(24 * time.Hour)

		layout := "2006-01-02"
		fromDateTime, err := time.Parse(layout, filter.FromDate)
		if err != nil {
			return nil, nil, err
		}

		diffHours := currentDate.Sub(fromDateTime.Truncate(24 * time.Hour)).Hours()
		if diffHours > (24 * 100) {
			return nil, nil, fmt.Errorf("cannot filter txs which are older than 100 days")
		}

		fromDate := fromDateTime.Truncate(24 * time.Hour).UnixNano()

		toDateTime, err := time.Parse(layout, filter.ToDate)
		if err != nil {
			return nil, nil, err
		}

		diffHours = currentDate.Sub(toDateTime.Truncate(24 * time.Hour)).Hours()
		if diffHours > (24 * 100) {
			return nil, nil, fmt.Errorf("cannot filter txs which are older than 100 days")
		}

		toDate := toDateTime.Truncate(24 * time.Hour).Add(24 * time.Hour).UnixNano()
		//

		switch filter.TxType {
		case ALL:
			// include internal txs and token transfers
			stmtBuilder = stmtBuilder.Where(
				"((view_account_transactions.is_internal_tx = false AND view_account_transactions.account = ?) OR "+
					"(view_account_transactions.account = ? AND view_account_transactions.is_internal_tx = true AND "+
					"(view_account_transactions.from_address = view_account_transaction_data.from_address AND view_account_transactions.to_address = view_account_transaction_data.to_address))) "+
					"AND (view_account_transactions.block_time >= ? AND view_account_transactions.block_time < ?)",
				filter.Account,
				filter.Account,
				fromDate,
				toDate,
			)
		case SEND:
			stmtBuilder = stmtBuilder.Where(
				"view_account_transactions.is_internal_tx = ? AND view_account_transactions.account = ? "+
					"AND view_account_transactions.from_address = ? "+
					"AND (view_account_transactions.block_time >= ? AND view_account_transactions.block_time < ?) "+
					"AND (view_account_transaction_data.reward_tx_type = 'send' OR view_account_transaction_data.reward_tx_type = 'transfer')",
				false,
				filter.Account,
				addressHash,
				fromDate,
				toDate,
			)
		case RECEIVE:
			stmtBuilder = stmtBuilder.Where(
				"view_account_transactions.is_internal_tx = ? AND view_account_transactions.account = ? "+
					"AND view_account_transactions.to_address = ? "+
					"AND (view_account_transactions.block_time >= ? AND view_account_transactions.block_time < ?) "+
					"AND (view_account_transaction_data.reward_tx_type = 'send' OR view_account_transaction_data.reward_tx_type = 'transfer')",
				false,
				filter.Account,
				addressHash,
				fromDate,
				toDate,
			)
		case REWARD:
			if filter.FromAddress == "" {
				stmtBuilder = stmtBuilder.Where(
					"view_account_transactions.is_internal_tx = ? AND view_account_transactions.account = ? "+
						"AND (view_account_transactions.block_time >= ? AND view_account_transactions.block_time < ?) "+
						"AND view_account_transaction_data.reward_tx_type = 'sendReward' "+
						"AND (view_account_transactions.from_address = view_account_transaction_data.from_address AND view_account_transactions.to_address = view_account_transaction_data.to_address)",
					true,
					filter.Account,
					fromDate,
					toDate,
				)
			} else {
				stmtBuilder = stmtBuilder.Where(
					"view_account_transactions.is_internal_tx = ? AND view_account_transactions.account = ? "+
						"AND (view_account_transactions.block_time >= ? AND view_account_transactions.block_time < ?) "+
						"AND view_account_transaction_data.reward_tx_type = 'sendReward' "+
						"AND view_account_transactions.from_address = ? "+
						"AND (view_account_transactions.from_address = view_account_transaction_data.from_address AND view_account_transactions.to_address = view_account_transaction_data.to_address)",
					true,
					filter.Account,
					fromDate,
					toDate,
					filter.FromAddress,
				)
			}
		case EXCHANGE_COUPON:
			stmtBuilder = stmtBuilder.Where(
				"view_account_transactions.is_internal_tx = ? AND view_account_transactions.account = ? "+
					"AND (view_account_transactions.block_time >= ? AND view_account_transactions.block_time < ?) "+
					"AND (view_account_transaction_data.reward_tx_type = 'exchange' OR view_account_transaction_data.reward_tx_type = 'exchangeWithValue')",
				false,
				filter.Account,
				fromDate,
				toDate,
			)
		case SAVING:
			stmtBuilder = stmtBuilder.Where(
				"view_account_transactions.is_internal_tx = ? AND view_account_transactions.account = ? "+
					"AND (view_account_transactions.block_time >= ? AND view_account_transactions.block_time < ?) "+
					"AND CAST(view_account_transactions.message_types AS VARCHAR) LIKE '%"+
					"elegat"+
					"%'",
				false,
				filter.Account,
				fromDate,
				toDate,
			)
		}

		// filter by txs status
		if filter.Status == "success" {
			stmtBuilder = stmtBuilder.Where(
				"view_account_transactions.success = ?",
				true,
			)
		} else if filter.Status == "failed" {
			stmtBuilder = stmtBuilder.Where(
				"view_account_transactions.success = ?",
				false,
			)
		}
	}

	if order.Id == view.ORDER_DESC {
		stmtBuilder = stmtBuilder.OrderBy("view_account_transactions.id DESC")
	} else {
		stmtBuilder = stmtBuilder.OrderBy("view_account_transactions.id")
	}

	rDbPagination := rdb.NewRDbPaginationBuilder(
		pagination,
		accountMessagesView.rdb,
	).WithCustomTotalQueryFn(
		func(rdbHandle *rdb.Handle, _ sq.SelectBuilder) (int64, error) {
			identity := ""
			if filter.Memo != "" {
				identity = fmt.Sprintf("%s/%s:-", filter.Account, filter.Memo)
			} else {
				identity = fmt.Sprintf("%s:-", filter.Account)
			}
			/*
				if filter.TxType == "" && filter.IncludingInternalTx == "true" {
					rawQuery := fmt.Sprintf(
						"SELECT "+
							"(SELECT coalesce(COUNT(*), 0) FROM (SELECT DISTINCT view_account_transactions.id FROM view_account_transactions "+
							"INNER JOIN view_account_transaction_data ON "+
							"view_account_transactions.block_height = view_account_transaction_data.block_height AND "+
							"view_account_transactions.transaction_hash = view_account_transaction_data.hash "+
							"WHERE account = '%s' AND is_internal_tx = true AND "+
							"(view_account_transactions.from_address = view_account_transaction_data.from_address AND view_account_transactions.to_address = view_account_transaction_data.to_address)) AS temp) + "+
							"(SELECT coalesce(SUM(total), 0) FROM view_account_transactions_total "+
							"WHERE identity = '%s') "+
							"AS total", filter.Account, identity)
					var total int64
					err := rdbHandle.QueryRow(rawQuery).Scan(&total)
					if err != nil {
						return int64(0), fmt.Errorf("error count account txs with reward tx type filter: %v: %w", err, rdb.ErrQuery)
					}
					return total, nil
				} else {
			*/
			totalView := NewAccountTransactionsTotal(rdbHandle)
			total, err := totalView.FindBy(identity)
			if err != nil {
				return int64(0), err
			}
			return total, nil
			//}
		},
	).BuildStmt(stmtBuilder)

	sql, sqlArgs, err := rDbPagination.ToStmtBuilder().ToSql()
	if err != nil {
		return nil, nil, fmt.Errorf(
			"error building account transactions select SQL: %v, %w", err, rdb.ErrBuildSQLStmt,
		)
	}

	rowsResult, err := accountMessagesView.rdb.Query(sql, sqlArgs...)
	if err != nil {
		return nil, nil, fmt.Errorf("error executing account transactions select SQL: %v: %w", err, rdb.ErrQuery)
	}
	defer rowsResult.Close()

	accountMessages := make([]AccountTransactionReadRow, 0)
	for rowsResult.Next() {
		var accountMessage AccountTransactionReadRow
		var feeJSON *string
		var messagesJSON *string
		var messageTypesJSON *string
		blockTimeReader := accountMessagesView.rdb.NtotReader()

		if err = rowsResult.Scan(
			&accountMessage.Id,
			&accountMessage.Account,
			&accountMessage.BlockHeight,
			&accountMessage.BlockHash,
			blockTimeReader.ScannableArg(),
			&accountMessage.Hash,
			&accountMessage.Success,

			&accountMessage.Code,
			&accountMessage.Log,
			&feeJSON,
			&accountMessage.FeePayer,
			&accountMessage.FeeGranter,
			&accountMessage.GasWanted,
			&accountMessage.GasUsed,
			&accountMessage.Memo,
			&accountMessage.TimeoutHeight,
			&messageTypesJSON,
			&messagesJSON,
		); err != nil {
			if errors.Is(err, rdb.ErrNoRows) {
				return nil, nil, rdb.ErrNoRows
			}
			return nil, nil, fmt.Errorf("error scanning account transaction row: %v: %w", err, rdb.ErrQuery)
		}
		blockTime, parseErr := blockTimeReader.Parse()
		if parseErr != nil {
			return nil, nil, fmt.Errorf(
				"error parsing account transaction block time: %v: %w", parseErr, rdb.ErrQuery,
			)
		}
		accountMessage.BlockTime = *blockTime

		var fee coin.Coins
		if unmarshalErr := jsoniter.UnmarshalFromString(*feeJSON, &fee); unmarshalErr != nil {
			return nil, nil, fmt.Errorf("error unmarshalling account transaction fee JSON: %v: %w", unmarshalErr, rdb.ErrQuery)
		}
		accountMessage.Fee = fee

		var messageTypes []string
		if unmarshalErr := jsoniter.UnmarshalFromString(*messageTypesJSON, &messageTypes); unmarshalErr != nil {
			return nil, nil, fmt.Errorf("error unmarshalling account transaction message types JSON: %v: %w", unmarshalErr, rdb.ErrQuery)
		}
		accountMessage.MessageTypes = messageTypes

		var messages []TransactionRowMessage
		if unmarshalErr := jsoniter.UnmarshalFromString(*messagesJSON, &messages); unmarshalErr != nil {
			return nil, nil, fmt.Errorf("error unmarshalling account transaction messages JSON: %v: %w", unmarshalErr, rdb.ErrQuery)
		}
		accountMessage.Messages = messages

		accountMessages = append(accountMessages, accountMessage)
	}

	paginationResult, err := rDbPagination.Result()
	if err != nil {
		return nil, nil, fmt.Errorf("error preparing pagination result: %v", err)
	}

	return accountMessages, paginationResult, nil
}

type AccountTransactionRecord struct {
	Row      AccountTransactionBaseRow
	Accounts []string
}

type AccountTransactionBaseRow struct {
	Id           int64           `json:"id"`
	Account      string          `json:"account"`
	BlockHeight  int64           `json:"blockHeight"`
	BlockHash    string          `json:"blockHash"`
	BlockTime    utctime.UTCTime `json:"blockTime"`
	Hash         string          `json:"hash"`
	MessageTypes []string        `json:"messageTypes"`
	Success      bool            `json:"success"`
	FromAddress  string          `json:"from_address,omitempty"`
	ToAddress    string          `json:"to_address,omitempty"`
	IsInternalTx bool            `json:"is_internal_tx,omitempty"`
	TxIndex      int             `json:"tx_index,omitempty"`
}

type AccountTransactionReadRow struct {
	AccountTransactionBaseRow

	Success       bool                    `json:"success"`
	Code          int                     `json:"code"`
	Log           string                  `json:"log"`
	Fee           coin.Coins              `json:"fee"`
	FeePayer      string                  `json:"feePayer"`
	FeeGranter    string                  `json:"feeGranter"`
	GasWanted     int                     `json:"gasWanted"`
	GasUsed       int                     `json:"gasUsed"`
	Memo          string                  `json:"memo"`
	TimeoutHeight int64                   `json:"timeoutHeight"`
	Messages      []TransactionRowMessage `json:"messages"`
}

type AccountTransactionsListFilter struct {
	// Required account filter
	Account string
	// Optional from address filter
	FromAddress string
	// Optional memo filter
	Memo string
	// Optional including internal txs filter
	IncludingInternalTx string
	// Optional tx type filter
	TxType string
	// Optional from date filter
	FromDate string
	// Optional to date filter
	ToDate string
	// Optional status filter
	Status string
}

type AccountTransactionsListOrder struct {
	Id view.ORDER
}
