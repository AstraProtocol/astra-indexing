package view

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/AstraProtocol/astra-indexing/external/json"
	"github.com/AstraProtocol/astra-indexing/external/tmcosmosutils"
	"github.com/AstraProtocol/astra-indexing/usecase/coin"

	sq "github.com/Masterminds/squirrel"

	pagination_interface "github.com/AstraProtocol/astra-indexing/appinterface/pagination"
	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"

	jsoniter "github.com/json-iterator/go"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/external/utctime"
)

const EXCHANGE = "exchange"
const EXCHANGE_WITH_VALUE = "exchangeWithValue"
const SEND = "send"
const RECEIVE = "receive"

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

	if filter.IncludingInternalTx == "true" {
		if filter.Memo == "" && filter.RewardTxType == "" && filter.Direction == "" {
			stmtBuilder = stmtBuilder.Where(
				"view_account_transactions.account = ?", filter.Account,
			)
		}

		if filter.Memo != "" {
			stmtBuilder = stmtBuilder.Where(
				"view_account_transactions.account = ? AND view_account_transaction_data.memo = ?", filter.Account, filter.Memo,
			)
		}
	} else {
		if filter.Memo == "" && filter.RewardTxType == "" && filter.Direction == "" {
			stmtBuilder = stmtBuilder.Where(
				"view_account_transactions.is_internal_tx = ? AND view_account_transactions.account = ?", false, filter.Account,
			)
		}

		if filter.Memo != "" {
			stmtBuilder = stmtBuilder.Where(
				"view_account_transactions.is_internal_tx = ? AND view_account_transactions.account = ? AND view_account_transaction_data.memo = ?", false, filter.Account, filter.Memo,
			)
		}
	}

	if filter.RewardTxType != "" && filter.Direction == "" {
		if filter.RewardTxType == EXCHANGE {
			stmtBuilder = stmtBuilder.Where(
				"view_account_transactions.account = ? AND (view_account_transaction_data.reward_tx_type = ? OR view_account_transaction_data.reward_tx_type = ?)",
				filter.Account,
				EXCHANGE,
				EXCHANGE_WITH_VALUE,
			)
		} else {
			stmtBuilder = stmtBuilder.Where(
				"view_account_transactions.account = ? AND view_account_transaction_data.reward_tx_type = ?",
				filter.Account,
				filter.RewardTxType,
			)
		}
	}

	_, converted, err := tmcosmosutils.DecodeAddressToHex(filter.Account)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"error decode astra address to hex address: %v", err,
		)
	}
	evmAddressHash := strings.ToLower("0x" + hex.EncodeToString(converted))

	if filter.Direction != "" && filter.RewardTxType == "" {
		if filter.Direction == SEND {
			stmtBuilder = stmtBuilder.Where(
				"view_account_transactions.account = ? AND view_account_transactions.from_address = ?",
				filter.Account,
				evmAddressHash,
			)
		} else if filter.Direction == RECEIVE {
			stmtBuilder = stmtBuilder.Where(
				"view_account_transactions.account = ? AND view_account_transactions.to_address = ?",
				filter.Account,
				evmAddressHash,
			)
		}
	}

	if filter.Direction != "" && filter.RewardTxType != "" {
		if filter.RewardTxType == EXCHANGE {
			if filter.Direction == SEND {
				stmtBuilder = stmtBuilder.Where(
					"view_account_transactions.account = ? AND view_account_transactions.from_address = ? AND (view_account_transaction_data.reward_tx_type = ? OR view_account_transaction_data.reward_tx_type = ?)",
					filter.Account,
					evmAddressHash,
					EXCHANGE,
					EXCHANGE_WITH_VALUE,
				)
			} else if filter.Direction == RECEIVE {
				stmtBuilder = stmtBuilder.Where(
					"view_account_transactions.account = ? AND view_account_transactions.to_address = ? AND (view_account_transaction_data.reward_tx_type = ? OR view_account_transaction_data.reward_tx_type = ?)",
					filter.Account,
					evmAddressHash,
					EXCHANGE,
					EXCHANGE_WITH_VALUE,
				)
			}
		} else {
			if filter.Direction == SEND {
				stmtBuilder = stmtBuilder.Where(
					"view_account_transactions.account = ? AND view_account_transactions.from_address = ? AND view_account_transaction_data.reward_tx_type = ?",
					filter.Account,
					evmAddressHash,
					filter.RewardTxType,
				)
			} else if filter.Direction == RECEIVE {
				stmtBuilder = stmtBuilder.Where(
					"view_account_transactions.account = ? AND view_account_transactions.to_address = ? AND view_account_transaction_data.reward_tx_type = ?",
					filter.Account,
					evmAddressHash,
					filter.RewardTxType,
				)
			}
		}
	}

	if order.Id == view.ORDER_DESC {
		stmtBuilder = stmtBuilder.OrderBy("view_account_transactions.id DESC")
	} else {
		stmtBuilder = stmtBuilder.OrderBy("view_account_transactions.id")
	}

	var rDbPagination *rdb.RDbPaginationStmtBuilder
	if filter.Direction == "" && filter.RewardTxType == "" {
		rDbPagination = rdb.NewRDbPaginationBuilder(
			pagination,
			accountMessagesView.rdb,
		).WithCustomTotalQueryFn(
			func(rdbHandle *rdb.Handle, _ sq.SelectBuilder) (int64, error) {
				totalView := NewAccountTransactionsTotal(rdbHandle)

				identity := ""
				if filter.Memo != "" {
					identity = fmt.Sprintf("%s/%s:-", filter.Account, filter.Memo)
				} else {
					identity = fmt.Sprintf("%s:-", filter.Account)
				}

				total, err := totalView.FindBy(identity)
				if err != nil {
					return int64(0), err
				}
				return total, nil
			},
		).BuildStmt(stmtBuilder)
	} else {
		rawQuery := fmt.Sprintf("SELECT COUNT(*) "+
			"FROM view_account_transactions "+
			"INNER JOIN view_account_transaction_data "+
			"ON view_account_transactions.block_height = view_account_transaction_data.block_height "+
			"AND view_account_transactions.transaction_hash = view_account_transaction_data.hash "+
			"WHERE view_account_transactions.account = '%s' AND ", filter.Account)
		if filter.RewardTxType != "" && filter.Direction == "" {
			rDbPagination = rdb.NewRDbPaginationBuilder(
				pagination,
				accountMessagesView.rdb,
			).WithCustomTotalQueryFn(
				func(rdbHandle *rdb.Handle, _ sq.SelectBuilder) (int64, error) {
					filterQuery := ""
					if filter.RewardTxType == EXCHANGE {
						filterQuery = fmt.Sprintf(
							"(view_account_transaction_data.reward_tx_type = '%s' OR "+
								"view_account_transaction_data.reward_tx_type = '%s')",
							EXCHANGE,
							EXCHANGE_WITH_VALUE,
						)
					} else {
						filterQuery = fmt.Sprintf(
							"view_account_transaction_data.reward_tx_type = '%s'",
							filter.RewardTxType,
						)
					}
					var total int64
					err := rdbHandle.QueryRow(rawQuery + filterQuery).Scan(&total)
					if err != nil {
						return 0, fmt.Errorf("error count account txs with reward tx type filter: %v: %w", err, rdb.ErrQuery)
					}
					return total, nil
				},
			).BuildStmt(stmtBuilder)
		} else if filter.Direction != "" && filter.RewardTxType == "" {
			rDbPagination = rdb.NewRDbPaginationBuilder(
				pagination,
				accountMessagesView.rdb,
			).WithCustomTotalQueryFn(
				func(rdbHandle *rdb.Handle, _ sq.SelectBuilder) (int64, error) {
					filterQuery := ""
					if filter.Direction == SEND {
						filterQuery = fmt.Sprintf(
							"view_account_transactions.from_address = '%s'",
							evmAddressHash,
						)
					} else if filter.Direction == RECEIVE {
						filterQuery = fmt.Sprintf(
							"view_account_transactions.to_address = '%s'",
							evmAddressHash,
						)
					}
					var total int64
					err := rdbHandle.QueryRow(rawQuery + filterQuery).Scan(&total)
					if err != nil {
						return 0, fmt.Errorf("error count account txs with direction filter: %v: %w", err, rdb.ErrQuery)
					}
					return total, nil
				},
			).BuildStmt(stmtBuilder)
		} else if filter.Direction != "" && filter.RewardTxType != "" {
			rDbPagination = rdb.NewRDbPaginationBuilder(
				pagination,
				accountMessagesView.rdb,
			).WithCustomTotalQueryFn(
				func(rdbHandle *rdb.Handle, _ sq.SelectBuilder) (int64, error) {
					filterQuery := ""
					if filter.RewardTxType == EXCHANGE {
						if filter.Direction == SEND {
							filterQuery = fmt.Sprintf(
								"view_account_transactions.from_address = '%s' AND (view_account_transaction_data.reward_tx_type = '%s' OR view_account_transaction_data.reward_tx_type = '%s')",
								evmAddressHash,
								EXCHANGE,
								EXCHANGE_WITH_VALUE,
							)
						} else if filter.Direction == RECEIVE {
							filterQuery = fmt.Sprintf(
								"view_account_transactions.to_address = '%s' AND (view_account_transaction_data.reward_tx_type = '%s' OR view_account_transaction_data.reward_tx_type = '%s')",
								evmAddressHash,
								EXCHANGE,
								EXCHANGE_WITH_VALUE,
							)
						}
					} else {
						if filter.Direction == SEND {
							filterQuery = fmt.Sprintf(
								"view_account_transactions.from_address = '%s' AND view_account_transaction_data.reward_tx_type = '%s'",
								evmAddressHash,
								filter.RewardTxType,
							)
						} else if filter.Direction == RECEIVE {
							filterQuery = fmt.Sprintf(
								"view_account_transactions.to_address = '%s' AND view_account_transaction_data.reward_tx_type = '%s'",
								evmAddressHash,
								filter.RewardTxType,
							)
						}
					}
					var total int64
					err := rdbHandle.QueryRow(rawQuery + filterQuery).Scan(&total)
					if err != nil {
						return 0, fmt.Errorf("error count account txs with reward tx type and direction filter: %v: %w", err, rdb.ErrQuery)
					}
					return total, nil
				},
			).BuildStmt(stmtBuilder)
		}
	}

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
	Account      string          `json:"account,omitempty"`
	BlockHeight  int64           `json:"blockHeight"`
	BlockHash    string          `json:"blockHash"`
	BlockTime    utctime.UTCTime `json:"blockTime"`
	Hash         string          `json:"hash"`
	MessageTypes []string        `json:"messageTypes"`
	Success      bool            `json:"success"`
	FromAddress  string          `json:"from_address"`
	ToAddress    string          `json:"to_address"`
	IsInternalTx bool            `json:"is_internal_tx"`
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
	// Optional memo filter
	Memo string
	// Optional reward tx type filter
	RewardTxType string
	// Optional direction filter
	Direction string
	// Optional including internal txs filter
	IncludingInternalTx string
}

type AccountTransactionsListOrder struct {
	Id view.ORDER
}
