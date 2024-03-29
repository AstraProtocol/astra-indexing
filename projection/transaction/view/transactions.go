package view

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgtype"

	pagination_interface "github.com/AstraProtocol/astra-indexing/appinterface/pagination"
	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/external/json"
	"github.com/AstraProtocol/astra-indexing/external/utctime"
	evm_utils "github.com/AstraProtocol/astra-indexing/internal/evm"
	"github.com/AstraProtocol/astra-indexing/usecase/coin"
)

type BlockTransactions interface {
	InsertAll(transactions []TransactionRow) error
	Insert(transaction *TransactionRow) error
	FindByHash(txHash string) (*TransactionRow, error)
	FindByEvmHash(txEvmHash string) (*TransactionRow, error)
	GetTxsTypeByEvmHashes(evmHashes []string) ([]TransactionTxType, error)
	List(
		filter TransactionsListFilter,
		order TransactionsListOrder,
		pagination *pagination_interface.Pagination,
	) ([]TransactionRow, *pagination_interface.Result, error)
	Search(keyword string) ([]TransactionRow, error)
	Count() (int64, error)
	UpdateAll([]map[string]interface{}) error
}

// BlockTransactions projection view implemented by relational database
type BlockTransactionsView struct {
	rdb *rdb.Handle
}

func NewTransactionsView(handle *rdb.Handle) BlockTransactions {
	return &BlockTransactionsView{
		handle,
	}
}

func (transactionsView *BlockTransactionsView) InsertAll(transactions []TransactionRow) error {
	pendingRowCount := 0
	var stmtBuilder sq.InsertBuilder

	transactionCount := len(transactions)
	for i, transaction := range transactions {
		if pendingRowCount == 0 {
			stmtBuilder = transactionsView.rdb.StmtBuilder.Insert(
				"view_transactions",
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
				"fee_value",
				"fee_payer",
				"fee_granter",
				"gas_wanted",
				"gas_used",
				"from_address",
				"to_address",
				"memo",
				"timeout_height",
				"messages",
				"signers",
				"tx_type",
			)
		}
		var transactionMessagesJSON string
		var marshalErr error
		if transactionMessagesJSON, marshalErr = json.MarshalToString(transaction.Messages); marshalErr != nil {
			return fmt.Errorf(
				"error JSON marshalling block transation messages for insertion: %v: %w", marshalErr, rdb.ErrBuildSQLStmt,
			)
		}

		var feeJSON string
		if feeJSON, marshalErr = json.MarshalToString(transaction.Fee); marshalErr != nil {
			return fmt.Errorf(
				"error JSON marshalling block transation fee for insertion: %v: %w", marshalErr, rdb.ErrBuildSQLStmt,
			)
		}
		var feeValue pgtype.Numeric
		feeValue.Set(transaction.Fee.AmountOf("aastra").String())

		var signersJSON string
		if signersJSON, marshalErr = json.MarshalToString(transaction.Signers); marshalErr != nil {
			return fmt.Errorf(
				"error JSON marshalling block transation signers for insertion: %v: %w", marshalErr, rdb.ErrBuildSQLStmt,
			)
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
			feeValue,
			transaction.FeePayer,
			transaction.FeeGranter,
			transaction.GasWanted,
			transaction.GasUsed,
			transaction.FromAddress,
			transaction.ToAddress,
			transaction.Memo,
			transaction.TimeoutHeight,
			transactionMessagesJSON,
			signersJSON,
			transaction.TxType,
		)
		pendingRowCount += 1

		// Postgres has a limit of 65536 parameters.
		if pendingRowCount == 500 || i+1 == transactionCount {
			sql, sqlArgs, err := stmtBuilder.ToSql()
			if err != nil {
				return fmt.Errorf("error building block transactions insertion sql: %v: %w", err, rdb.ErrBuildSQLStmt)
			}

			result, err := transactionsView.rdb.Exec(sql, sqlArgs...)
			if err != nil {
				return fmt.Errorf("error inserting block transaction into the table: %v: %w", err, rdb.ErrWrite)
			}
			if result.RowsAffected() != int64(pendingRowCount) {
				return fmt.Errorf("error inserting block transaction into the table: no rows inserted: %w", rdb.ErrWrite)
			}
			pendingRowCount = 0
		}
	}

	return nil
}

func (transactionsView *BlockTransactionsView) Insert(transaction *TransactionRow) error {
	var err error

	var sql string
	sql, _, err = transactionsView.rdb.StmtBuilder.Insert(
		"view_transactions",
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
		"fee_value",
		"fee_payer",
		"fee_granter",
		"gas_wanted",
		"gas_used",
		"from_address",
		"memo",
		"timeout_height",
		"messages",
		"signers",
	).Values("?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?").ToSql()
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

	var feeValue pgtype.Numeric
	feeValue.Set(transaction.Fee.AmountOf("aastra").String())

	var signersJSON string
	if signersJSON, err = json.MarshalToString(transaction.Signers); err != nil {
		return fmt.Errorf("error JSON marshalling block transation signers for insertion: %v: %w", err, rdb.ErrBuildSQLStmt)
	}

	result, err := transactionsView.rdb.Exec(sql,
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
		feeValue,
		transaction.FeePayer,
		transaction.FeeGranter,
		transaction.GasWanted,
		transaction.GasUsed,
		transaction.FromAddress,
		transaction.Memo,
		transaction.TimeoutHeight,
		transactionMessagesJSON,
		signersJSON,
	)
	if err != nil {
		return fmt.Errorf("error inserting block transaction into the table: %v: %w", err, rdb.ErrWrite)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("error inserting block transaction into the table: no rows inserted: %w", rdb.ErrWrite)
	}

	return nil
}

func (transactionsView *BlockTransactionsView) FindByHash(txHash string) (*TransactionRow, error) {
	var err error

	selectStmtBuilder := transactionsView.rdb.StmtBuilder.Select(
		"block_height",
		"block_hash",
		"block_time",
		"hash",
		"evm_hash",
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
		"signers",
	).From(
		"view_transactions",
	).Where(
		"hash = ?", txHash,
	).OrderBy("id DESC")

	sql, sqlArgs, err := selectStmtBuilder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("error building transactions selection sql: %v: %w", err, rdb.ErrPrepare)
	}

	var transaction TransactionRow
	var feeJSON *string
	var messagesJSON *string
	var signersJSON *string
	blockTimeReader := transactionsView.rdb.NtotReader()

	if err = transactionsView.rdb.QueryRow(sql, sqlArgs...).Scan(
		&transaction.BlockHeight,
		&transaction.BlockHash,
		blockTimeReader.ScannableArg(),
		&transaction.Hash,
		&transaction.EvmHash,
		&transaction.Success,
		&transaction.Code,
		&transaction.Log,
		&feeJSON,
		&transaction.FeePayer,
		&transaction.FeeGranter,
		&transaction.GasWanted,
		&transaction.GasUsed,
		&transaction.Memo,
		&transaction.TimeoutHeight,
		&messagesJSON,
		&signersJSON,
	); err != nil {
		if errors.Is(err, rdb.ErrNoRows) {
			return nil, rdb.ErrNoRows
		}
		return nil, fmt.Errorf("error scanning transaction row: %v: %w", err, rdb.ErrQuery)
	}
	blockTime, parseErr := blockTimeReader.Parse()
	if parseErr != nil {
		return nil, fmt.Errorf("error parsing transaction block time: %v: %w", parseErr, rdb.ErrQuery)
	}
	transaction.BlockTime = *blockTime

	var fee coin.Coins
	if unmarshalErr := json.UnmarshalFromString(*feeJSON, &fee); unmarshalErr != nil {
		return nil, fmt.Errorf("error unmarshalling transaction fee JSON: %v: %w", unmarshalErr, rdb.ErrQuery)
	}
	transaction.Fee = fee

	var messages []TransactionRowMessage
	if unmarshalErr := json.UnmarshalFromString(*messagesJSON, &messages); unmarshalErr != nil {
		return nil, fmt.Errorf("error unmarshalling transaction messages JSON: %v: %w", unmarshalErr, rdb.ErrQuery)
	}
	transaction.Messages = messages

	var signers []TransactionRowSigner
	if unmarshalErr := json.UnmarshalFromString(*signersJSON, &signers); unmarshalErr != nil {
		return nil, fmt.Errorf("error unmarshalling transaction signers JSON: %v: %w", unmarshalErr, rdb.ErrQuery)
	}
	transaction.Signers = signers

	return &transaction, nil
}

func (transactionsView *BlockTransactionsView) FindByEvmHash(txEvmHash string) (*TransactionRow, error) {
	var err error

	selectStmtBuilder := transactionsView.rdb.StmtBuilder.Select(
		"block_height",
		"block_hash",
		"block_time",
		"hash",
		"evm_hash",
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
		"signers",
	).From(
		"view_transactions",
	).Where(
		"evm_hash = ?", txEvmHash,
	).OrderBy("id DESC")

	sql, sqlArgs, err := selectStmtBuilder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("error building transactions selection sql: %v: %w", err, rdb.ErrPrepare)
	}

	var transaction TransactionRow
	var feeJSON *string
	var messagesJSON *string
	var signersJSON *string
	blockTimeReader := transactionsView.rdb.NtotReader()

	if err = transactionsView.rdb.QueryRow(sql, sqlArgs...).Scan(
		&transaction.BlockHeight,
		&transaction.BlockHash,
		blockTimeReader.ScannableArg(),
		&transaction.Hash,
		&transaction.EvmHash,
		&transaction.Success,
		&transaction.Code,
		&transaction.Log,
		&feeJSON,
		&transaction.FeePayer,
		&transaction.FeeGranter,
		&transaction.GasWanted,
		&transaction.GasUsed,
		&transaction.Memo,
		&transaction.TimeoutHeight,
		&messagesJSON,
		&signersJSON,
	); err != nil {
		if errors.Is(err, rdb.ErrNoRows) {
			return nil, rdb.ErrNoRows
		}
		return nil, fmt.Errorf("error scanning transaction row: %v: %w", err, rdb.ErrQuery)
	}
	blockTime, parseErr := blockTimeReader.Parse()
	if parseErr != nil {
		return nil, fmt.Errorf("error parsing transaction block time: %v: %w", parseErr, rdb.ErrQuery)
	}
	transaction.BlockTime = *blockTime

	var fee coin.Coins
	if unmarshalErr := json.UnmarshalFromString(*feeJSON, &fee); unmarshalErr != nil {
		return nil, fmt.Errorf("error unmarshalling transaction fee JSON: %v: %w", unmarshalErr, rdb.ErrQuery)
	}
	transaction.Fee = fee

	var messages []TransactionRowMessage
	if unmarshalErr := json.UnmarshalFromString(*messagesJSON, &messages); unmarshalErr != nil {
		return nil, fmt.Errorf("error unmarshalling transaction messages JSON: %v: %w", unmarshalErr, rdb.ErrQuery)
	}
	transaction.Messages = messages

	var signers []TransactionRowSigner
	if unmarshalErr := json.UnmarshalFromString(*signersJSON, &signers); unmarshalErr != nil {
		return nil, fmt.Errorf("error unmarshalling transaction signers JSON: %v: %w", unmarshalErr, rdb.ErrQuery)
	}
	transaction.Signers = signers

	return &transaction, nil
}

func (transactionsView *BlockTransactionsView) GetTxsTypeByEvmHashes(evmHashes []string) ([]TransactionTxType, error) {
	inValue := ""
	for index, evmHash := range evmHashes {
		if index == 0 {
			inValue += fmt.Sprintf("'%s'", evmHash)
		} else {
			inValue += fmt.Sprintf(",'%s'", evmHash)
		}
	}
	rawQuery := fmt.Sprintf("SELECT evm_hash, tx_type "+
		"FROM view_transactions "+
		"WHERE evm_hash IN(%s)", inValue)

	transactionTxTypes := make([]TransactionTxType, 0)
	rowsResult, err := transactionsView.rdb.Query(rawQuery)
	if err != nil {
		return nil, fmt.Errorf("error executing get txs type select SQL: %v: %w", err, rdb.ErrQuery)
	}
	defer rowsResult.Close()
	for rowsResult.Next() {
		var result TransactionTxType
		if err = rowsResult.Scan(
			&result.EvmHash,
			&result.TxType,
		); err != nil {
			if errors.Is(err, rdb.ErrNoRows) {
				return nil, rdb.ErrNoRows
			}
			return nil, fmt.Errorf("error scanning get txs type row: %v: %w", err, rdb.ErrQuery)
		}
		transactionTxTypes = append(transactionTxTypes, result)
	}
	return transactionTxTypes, nil
}

func (transactionsView *BlockTransactionsView) List(
	filter TransactionsListFilter,
	order TransactionsListOrder,
	pagination *pagination_interface.Pagination,
) ([]TransactionRow, *pagination_interface.Result, error) {
	stmtBuilder := transactionsView.rdb.StmtBuilder.Select(
		"block_height",
		"block_hash",
		"block_time",
		"hash",
		"evm_hash",
		"success",
		"code",
		"fee",
		"fee_payer",
		"fee_granter",
		"gas_wanted",
		"gas_used",
		"memo",
		"timeout_height",
		"messages",
		"signers",
	).From(
		"view_transactions",
	)

	if order.Height == view.ORDER_DESC {
		stmtBuilder = stmtBuilder.OrderBy("block_height DESC, id")
	} else {
		stmtBuilder = stmtBuilder.OrderBy("block_height, id")
	}

	if filter.MaybeBlockHeight != nil {
		stmtBuilder = stmtBuilder.Where("block_height = ?", *filter.MaybeBlockHeight)
	}

	rDbPagination := rdb.NewRDbPaginationBuilder(
		pagination,
		transactionsView.rdb,
	).WithCustomTotalQueryFn(
		func(rdbHandle *rdb.Handle, _ sq.SelectBuilder) (int64, error) {
			identity := "-"
			if filter.MaybeBlockHeight != nil {
				identity = strconv.FormatInt(*filter.MaybeBlockHeight, 10)
			}
			totalView := NewTransactionsTotalView(rdbHandle)
			total, err := totalView.FindBy(identity)
			if err != nil {
				return int64(0), err
			}
			return total, nil
		},
	).BuildStmt(stmtBuilder)
	sql, sqlArgs, err := rDbPagination.ToStmtBuilder().ToSql()
	if err != nil {
		return nil, nil, fmt.Errorf("error building transactions select SQL: %v, %w", err, rdb.ErrBuildSQLStmt)
	}

	rowsResult, err := transactionsView.rdb.Query(sql, sqlArgs...)
	if err != nil {
		return nil, nil, fmt.Errorf("error executing transactions select SQL: %v: %w", err, rdb.ErrQuery)
	}
	defer rowsResult.Close()

	transactions := make([]TransactionRow, 0)
	for rowsResult.Next() {
		var transaction TransactionRow
		var feeJSON *string
		var messagesJSON *string
		var signersJSON *string
		blockTimeReader := transactionsView.rdb.NtotReader()

		if err = rowsResult.Scan(
			&transaction.BlockHeight,
			&transaction.BlockHash,
			blockTimeReader.ScannableArg(),
			&transaction.Hash,
			&transaction.EvmHash,
			&transaction.Success,
			&transaction.Code,
			&feeJSON,
			&transaction.FeePayer,
			&transaction.FeeGranter,
			&transaction.GasWanted,
			&transaction.GasUsed,
			&transaction.Memo,
			&transaction.TimeoutHeight,
			&messagesJSON,
			&signersJSON,
		); err != nil {
			if errors.Is(err, rdb.ErrNoRows) {
				return nil, nil, rdb.ErrNoRows
			}
			return nil, nil, fmt.Errorf("error scanning transaction row: %v: %w", err, rdb.ErrQuery)
		}
		blockTime, parseErr := blockTimeReader.Parse()
		if parseErr != nil {
			return nil, nil, fmt.Errorf("error parsing transaction block time: %v: %w", parseErr, rdb.ErrQuery)
		}
		transaction.BlockTime = *blockTime

		var fee coin.Coins
		if unmarshalErr := json.UnmarshalFromString(*feeJSON, &fee); unmarshalErr != nil {
			return nil, nil, fmt.Errorf("error unmarshalling transaction fee JSON: %v: %w", unmarshalErr, rdb.ErrQuery)
		}
		transaction.Fee = fee

		var messages []TransactionRowMessage
		if unmarshalErr := json.UnmarshalFromString(*messagesJSON, &messages); unmarshalErr != nil {
			return nil, nil, fmt.Errorf("error unmarshalling transaction messages JSON: %v: %w", unmarshalErr, rdb.ErrQuery)
		}
		transaction.Messages = messages

		var signers []TransactionRowSigner
		if unmarshalErr := json.UnmarshalFromString(*signersJSON, &signers); unmarshalErr != nil {
			return nil, nil, fmt.Errorf("error unmarshalling transaction signers JSON: %v: %w", unmarshalErr, rdb.ErrQuery)
		}
		transaction.Signers = signers

		transactions = append(transactions, transaction)
	}

	paginationResult, err := rDbPagination.Result()
	if err != nil {
		return nil, nil, fmt.Errorf("error preparing pagination result: %v", err)
	}

	return transactions, paginationResult, nil
}

func (transactionsView *BlockTransactionsView) Search(keyword string) ([]TransactionRow, error) {
	var sql string
	var sqlArgs []interface{}
	var err error
	if evm_utils.IsHexTx(keyword) {
		keyword = strings.ToLower(keyword)
		sql, sqlArgs, err = transactionsView.rdb.StmtBuilder.Select(
			"block_height",
			"block_hash",
			"block_time",
			"hash",
			"evm_hash",
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
			"signers",
		).From(
			"view_transactions",
		).Where(
			"evm_hash = ?", keyword,
		).OrderBy(
			"block_height",
		).ToSql()
		if err != nil {
			return nil, fmt.Errorf("error building transactions select SQL: %v, %w", err, rdb.ErrBuildSQLStmt)
		}
	} else {
		keyword = strings.ToUpper(keyword)
		sql, sqlArgs, err = transactionsView.rdb.StmtBuilder.Select(
			"block_height",
			"block_hash",
			"block_time",
			"hash",
			"evm_hash",
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
			"signers",
		).From(
			"view_transactions",
		).Where(
			"hash = ?", keyword,
		).OrderBy(
			"block_height",
		).ToSql()
		if err != nil {
			return nil, fmt.Errorf("error building transactions select SQL: %v, %w", err, rdb.ErrBuildSQLStmt)
		}
	}

	var transaction TransactionRow
	var feeJSON *string
	var messagesJSON *string
	var signersJSON *string
	blockTimeReader := transactionsView.rdb.NtotReader()

	if err = transactionsView.rdb.QueryRow(sql, sqlArgs...).Scan(
		&transaction.BlockHeight,
		&transaction.BlockHash,
		blockTimeReader.ScannableArg(),
		&transaction.Hash,
		&transaction.EvmHash,
		&transaction.Success,
		&transaction.Code,
		&transaction.Log,
		&feeJSON,
		&transaction.FeePayer,
		&transaction.FeeGranter,
		&transaction.GasWanted,
		&transaction.GasUsed,
		&transaction.Memo,
		&transaction.TimeoutHeight,
		&messagesJSON,
		&signersJSON,
	); err != nil {
		if errors.Is(err, rdb.ErrNoRows) {
			return nil, rdb.ErrNoRows
		}
		return nil, fmt.Errorf("error scanning transaction row: %v: %w", err, rdb.ErrQuery)
	}

	blockTime, parseErr := blockTimeReader.Parse()
	if parseErr != nil {
		return nil, fmt.Errorf("error parsing transaction block time: %v: %w", parseErr, rdb.ErrQuery)
	}
	transaction.BlockTime = *blockTime

	var fee coin.Coins
	if unmarshalErr := json.UnmarshalFromString(*feeJSON, &fee); unmarshalErr != nil {
		return nil, fmt.Errorf("error unmarshalling transaction fee JSON: %v: %w", unmarshalErr, rdb.ErrQuery)
	}
	transaction.Fee = fee

	var messages []TransactionRowMessage
	if unmarshalErr := json.UnmarshalFromString(*messagesJSON, &messages); unmarshalErr != nil {
		return nil, fmt.Errorf("error unmarshalling transaction messages JSON: %v: %w", unmarshalErr, rdb.ErrQuery)
	}
	transaction.Messages = messages

	var signers []TransactionRowSigner
	if unmarshalErr := json.UnmarshalFromString(*signersJSON, &signers); unmarshalErr != nil {
		return nil, fmt.Errorf("error unmarshalling transaction signers JSON: %v: %w", unmarshalErr, rdb.ErrQuery)
	}
	transaction.Signers = signers

	transactions := make([]TransactionRow, 0)
	transactions = append(transactions, transaction)
	return transactions, nil
}

func (transactionsView *BlockTransactionsView) Count() (int64, error) {
	sql, _, err := transactionsView.rdb.StmtBuilder.Select("COUNT(1)").From(
		"view_transactions",
	).ToSql()
	if err != nil {
		return 0, fmt.Errorf("error building transactions count selection sql: %v", err)
	}

	result := transactionsView.rdb.QueryRow(sql)
	var count int64
	if err := result.Scan(&count); err != nil {
		return 0, fmt.Errorf("error scanning transactions count selection query: %v", err)
	}

	return count, nil
}

func (transactionsView *BlockTransactionsView) UpdateAll(mapValues []map[string]interface{}) error {
	tableName := "view_transactions"

	var updateValues string
	for index, mapValue := range mapValues {
		feeValue := mapValue["fee_value"].(string)

		var fee []map[string]string
		fee = append(fee, map[string]string{"denom": "aastra", "amount": feeValue})
		var feeJSON string
		var marshalErr error
		if feeJSON, marshalErr = json.MarshalToString(fee); marshalErr != nil {
			return fmt.Errorf(
				"error JSON marshalling evm tx fee for update: %v: %w", marshalErr, rdb.ErrBuildSQLStmt,
			)
		}

		evmHash := mapValue["evm_hash"].(string)
		success := mapValue["success"].(bool)
		if index == 0 {
			updateValues = fmt.Sprintf("('%s','%s'::DECIMAL,'%s',%v)", evmHash, feeValue, feeJSON, success)
		} else {
			updateValues = updateValues + fmt.Sprintf(",('%s','%s'::DECIMAL,'%s',%v)", evmHash, feeValue, feeJSON, success)
		}
	}
	bulkUpdate := fmt.Sprintf(`UPDATE %s SET fee_value=tmp.fee_value,fee=CAST(tmp.fee AS json),success=tmp.success `+
		`FROM (values %s) AS tmp (evm_hash,fee_value,fee,success) `+
		`WHERE %s.evm_hash=tmp.evm_hash;`, tableName, updateValues, tableName)

	execResult, err := transactionsView.rdb.Exec(bulkUpdate)
	if err != nil {
		return fmt.Errorf("error executing bulk update tx by evm hash SQL: %v", err)
	}
	if execResult.RowsAffected() == 0 {
		return errors.New("error executing bulk update tx by evm hash SQL: no rows updated")
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
	Status        string                  `json:"status"`
	Code          int                     `json:"code"`
	Log           string                  `json:"log"`
	Fee           coin.Coins              `json:"fee"`
	FeePayer      string                  `json:"feePayer"`
	FeeGranter    string                  `json:"feeGranter"`
	GasWanted     int                     `json:"gasWanted"`
	GasUsed       int                     `json:"gasUsed"`
	FromAddress   string                  `json:"fromAddress"`
	ToAddress     string                  `json:"toAddress"`
	Memo          string                  `json:"memo"`
	TimeoutHeight int64                   `json:"timeoutHeight"`
	Messages      []TransactionRowMessage `json:"messages"`
	Signers       []TransactionRowSigner  `json:"signers"`
	TxType        string                  `json:"txType"`
}

type TransactionRowMessage struct {
	Type    string      `json:"type"`
	Content interface{} `json:"content"`
	EvmType string      `json:"evmType"`
}

type TransactionsListFilter struct {
	MaybeBlockHeight *int64
}

type TransactionsListOrder struct {
	Height view.ORDER
}

type TransactionRowSigner struct {
	MaybeKeyInfo *TransactionRowSignerKeyInfo `json:"keyInfo"`

	Address         string `json:"address"`
	AccountSequence uint64 `json:"accountSequence"`
}

type TransactionRowSignerKeyInfo struct {
	Type           string   `json:"type"`
	IsMultiSig     bool     `json:"isMultiSig"`
	Pubkeys        []string `json:"pubkeys"`
	MaybeThreshold *int     `json:"threshold,omitempty"`
}

type TransactionTxType struct {
	EvmHash string `json:"evmHash"`
	TxType  string `json:"txType"`
}
