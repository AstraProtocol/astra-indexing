package view

import (
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"

	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/external/json"
	"github.com/AstraProtocol/astra-indexing/external/utctime"
	"github.com/AstraProtocol/astra-indexing/usecase/coin"
)

// AccountTransactionData projection view implemented by relational database
type AccountTransactionData struct {
	rdb *rdb.Handle
}

func NewAccountTransactionData(handle *rdb.Handle) *AccountTransactionData {
	return &AccountTransactionData{
		handle,
	}
}

func (transactionsView *AccountTransactionData) InsertAll(transactions []TransactionRow) error {
	pendingRowCount := 0
	var stmtBuilder sq.InsertBuilder

	transactionCount := len(transactions)
	for i, transaction := range transactions {
		if pendingRowCount == 0 {
			stmtBuilder = transactionsView.rdb.StmtBuilder.Insert(
				"view_account_transaction_data",
			).Columns(
				"block_height",
				"block_hash",
				"block_time",
				"hash",
				"evm_hash",
				"index",
				"success",
				"code",
				"log",
				"fee",
				"fee_payer",
				"fee_granter",
				"gas_wanted",
				"gas_used",
				"memo",
				"timeout_height",
				"messages",
			)
		}
		transactionMessagesJSON, err := json.MarshalToString(transaction.Messages)
		if err != nil {
			return fmt.Errorf("error JSON marshalling block transation messages for insertion: %v: %w", err, rdb.ErrBuildSQLStmt)
		}

		feeJSON, err := json.MarshalToString(transaction.Fee)
		if err != nil {
			return fmt.Errorf("error JSON marshalling block transation fee for insertion: %v: %w", err, rdb.ErrBuildSQLStmt)
		}

		stmtBuilder = stmtBuilder.Values(
			transaction.BlockHeight,
			transaction.BlockHash,
			transactionsView.rdb.Tton(&transaction.BlockTime),
			transaction.Hash,
			transaction.EvmHash,
			transaction.Index,
			transaction.Success,
			transaction.Code,
			transaction.Log,
			feeJSON,
			transaction.FeePayer,
			transaction.FeeGranter,
			transaction.GasWanted,
			transaction.GasUsed,
			transaction.Memo,
			transaction.TimeoutHeight,
			transactionMessagesJSON,
		)
		pendingRowCount += 1

		// Postgres has a limit of 65536 parameters.
		if pendingRowCount == 500 || i+1 == transactionCount {
			sql, sqlArgs, err := stmtBuilder.ToSql()
			if err != nil {
				return fmt.Errorf("error building account transactions data insertion sql: %v: %w", err, rdb.ErrBuildSQLStmt)
			}
			result, err := transactionsView.rdb.Exec(sql, sqlArgs...)
			if err != nil {
				return fmt.Errorf("error inserting block transaction into the table: %v: %w", err, rdb.ErrWrite)
			}
			if result.RowsAffected() != int64(pendingRowCount) {
				return fmt.Errorf("error inserting account transaction data into the table: mismatch rows inserted: %w", rdb.ErrWrite)
			}
			pendingRowCount = 0
		}
	}

	return nil
}

func (transactionsView *AccountTransactionData) Insert(transaction *TransactionRow) error {
	var err error

	var sql string
	sql, _, err = transactionsView.rdb.StmtBuilder.Insert(
		"view_account_transaction_data",
	).Columns(
		"block_height",
		"block_hash",
		"block_time",
		"hash",
		"index",
		"success",
		"code",
		"log",
		"fee",
		"fee_payer",
		"fee_granter",
		"gas_wanted",
		"gas_used",
		"memo",
		"timeout_height",
		"messages",
	).Values("?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?").ToSql()
	if err != nil {
		return fmt.Errorf("error building block transactions insertion sql: %v: %w", err, rdb.ErrBuildSQLStmt)
	}

	var transactionMessagesJSON string
	if transactionMessagesJSON, err = json.MarshalToString(transaction.Messages); err != nil {
		return fmt.Errorf("error JSON marshalling block transation messages for insertion: %v: %w", err, rdb.ErrBuildSQLStmt)
	}

	var feeJSON string
	if feeJSON, err = json.MarshalToString(transaction.Fee); err != nil {
		return fmt.Errorf("error JSON marshalling block transation fee for insertion: %v: %w", err, rdb.ErrBuildSQLStmt)
	}

	result, err := transactionsView.rdb.Exec(sql,
		transaction.BlockHeight,
		transaction.BlockHash,
		transactionsView.rdb.Tton(&transaction.BlockTime),
		transaction.Hash,
		transaction.Index,
		transaction.Success,
		transaction.Code,
		transaction.Log,
		feeJSON,
		transaction.FeePayer,
		transaction.FeeGranter,
		transaction.GasWanted,
		transaction.GasUsed,
		transaction.Memo,
		transaction.TimeoutHeight,
		transactionMessagesJSON,
	)
	if err != nil {
		return fmt.Errorf("error inserting block transaction into the table: %v: %w", err, rdb.ErrWrite)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("error inserting block transaction into the table: no rows inserted: %w", rdb.ErrWrite)
	}

	return nil
}

func (transactionsView *AccountTransactionData) UpdateAll(mapValues []map[string]interface{}) error {
	tableName := "view_account_transaction_data"

	var updateValues string
	for index, mapValue := range mapValues {
		feeValue := mapValue["fee_value"].(string)

		var fee []map[string]string
		fee = append(fee, map[string]string{"denom": "aastra", "amount": feeValue})
		var feeJSON string
		var marshalErr error
		if feeJSON, marshalErr = json.MarshalToString(fee); marshalErr != nil {
			return fmt.Errorf(
				"error JSON marshalling account evm tx fee data for update: %v: %w", marshalErr, rdb.ErrBuildSQLStmt,
			)
		}

		evmHash := mapValue["evm_hash"].(string)
		success := mapValue["success"].(bool)
		if index == 0 {
			updateValues = fmt.Sprintf("('%s','%s',%v)", evmHash, feeJSON, success)
		} else {
			updateValues = updateValues + fmt.Sprintf(",('%s','%s',%v)", evmHash, feeJSON, success)
		}
	}
	bulkUpdate := fmt.Sprintf(`UPDATE %s SET fee_value=tmp.fee_value,fee=CAST(tmp.fee AS json),success=tmp.success `+
		`FROM (values %s) AS tmp (evm_hash,fee,success) `+
		`WHERE %s.evm_hash=tmp.evm_hash;`, tableName, updateValues, tableName)

	execResult, err := transactionsView.rdb.Exec(bulkUpdate)
	if err != nil {
		return fmt.Errorf("error executing bulk update account tx data by evm hash SQL: %v", err)
	}
	if execResult.RowsAffected() == 0 {
		return errors.New("error executing bulk update account tx data by evm hash SQL: no rows updated")
	}

	return nil
}

type TransactionRow struct {
	BlockHeight   int64                   `json:"blockHeight"`
	BlockHash     string                  `json:"blockHash"`
	BlockTime     utctime.UTCTime         `json:"blockTime"`
	Hash          string                  `json:"hash"`
	EvmHash       string                  `json:"evmHash"`
	Index         int                     `json:"index"`
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

type TransactionRowMessage struct {
	Type    string      `json:"type"`
	EvmType string      `json:"evmType"`
	Content interface{} `json:"content"`
}

type TransactionsListFilter struct {
	MaybeBlockHeight *int64
}

type TransactionsListOrder struct {
	Height view.ORDER
}
