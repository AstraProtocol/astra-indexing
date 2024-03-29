package view

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/AstraProtocol/astra-indexing/external/json"
	"github.com/AstraProtocol/astra-indexing/usecase/coin"

	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"

	"github.com/AstraProtocol/astra-indexing/appinterface/pagination"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	_ "github.com/AstraProtocol/astra-indexing/test/factory"
)

type Accounts interface {
	Upsert(*AccountRow) error
	TotalAccount() (int64, error)
	FindBy(*AccountIdentity) (*AccountRow, error)
	List(AccountsListOrder, *pagination.Pagination) ([]AccountRow, *pagination.Result, error)
}

type AccountsView struct {
	rdb *rdb.Handle
}

type AccountsListOrder struct {
	AccountAddress view.ORDER
}

type AccountIdentity struct {
	Address string
}

func NewAccountsView(handle *rdb.Handle) Accounts {
	return &AccountsView{
		handle,
	}
}

func (accountsView *AccountsView) Upsert(account *AccountRow) error {
	sql, sqlArgs, err := accountsView.rdb.StmtBuilder.
		Insert(
			"view_accounts",
		).
		Columns(
			"address",
			"account_type",
			"name",
			"pubkey",
			"account_number",
			"sequence_number",
			"balance",
		).
		Values(
			account.Address,
			account.Type,
			account.MaybeName,
			account.MaybePubkey,
			account.AccountNumber,
			account.SequenceNumber,
			json.MustMarshalToString(account.Balance),
		).
		Suffix("ON CONFLICT(address) DO UPDATE SET balance = EXCLUDED.balance").
		ToSql()

	if err != nil {
		return fmt.Errorf("error building accounts insertion sql: %v: %w", err, rdb.ErrBuildSQLStmt)
	}

	result, err := accountsView.rdb.Exec(sql, sqlArgs...)
	if err != nil {
		return fmt.Errorf("error inserting account into the table: %v: %w", err, rdb.ErrWrite)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("error inserting account into the table: no rows inserted: %w", rdb.ErrWrite)
	}

	return nil
}

func (accountsView *AccountsView) TotalAccount() (int64, error) {
	var err error

	selectStmtBuilder := accountsView.rdb.StmtBuilder.Select(
		"MAX(account_number)",
	).From("view_accounts")

	sql, sqlArgs, err := selectStmtBuilder.ToSql()
	if err != nil {
		return -1, fmt.Errorf("error building total account selection sql: %v: %w", err, rdb.ErrPrepare)
	}

	var totalAccount int64
	if err = accountsView.rdb.QueryRow(sql, sqlArgs...).Scan(
		&totalAccount,
	); err != nil {
		if errors.Is(err, rdb.ErrNoRows) {
			return -1, rdb.ErrNoRows
		}
		return -1, fmt.Errorf("error scanning account row: %v: %w", err, rdb.ErrQuery)
	}

	return totalAccount, nil
}

func (accountsView *AccountsView) FindBy(identity *AccountIdentity) (*AccountRow, error) {
	var err error

	selectStmtBuilder := accountsView.rdb.StmtBuilder.Select(
		"address",
		"account_type",
		"name",
		"pubkey",
	).From("view_accounts")

	selectStmtBuilder = selectStmtBuilder.Where("address = ?", identity.Address)

	sql, sqlArgs, err := selectStmtBuilder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("error building account selection sql: %v: %w", err, rdb.ErrPrepare)
	}

	var account AccountRow
	if err = accountsView.rdb.QueryRow(sql, sqlArgs...).Scan(
		&account.Address,
		&account.Type,
		&account.MaybeName,
		&account.MaybePubkey,
	); err != nil {
		if errors.Is(err, rdb.ErrNoRows) {
			return nil, rdb.ErrNoRows
		}
		return nil, fmt.Errorf("error scanning account row: %v: %w", err, rdb.ErrQuery)
	}

	return &account, nil
}

func (accountsView *AccountsView) List(
	order AccountsListOrder,
	pagination *pagination.Pagination,
) ([]AccountRow, *pagination.Result, error) {
	stmtBuilder := accountsView.rdb.StmtBuilder.Select(
		"address",
		"account_type",
		"name",
		"pubkey",
		"account_number",
		"sequence_number",
		"balance",
	).From(
		"view_accounts",
	)

	if order.AccountAddress == view.ORDER_DESC {
		stmtBuilder = stmtBuilder.OrderBy("address DESC")
	} else {
		stmtBuilder = stmtBuilder.OrderBy("address")
	}

	rDbPagination := rdb.NewRDbPaginationBuilder(
		pagination,
		accountsView.rdb,
	).BuildStmt(stmtBuilder)
	sql, sqlArgs, err := rDbPagination.ToStmtBuilder().ToSql()
	if err != nil {
		return nil, nil, fmt.Errorf("error building accounts select SQL: %v, %w", err, rdb.ErrBuildSQLStmt)
	}

	rowsResult, err := accountsView.rdb.Query(sql, sqlArgs...)
	if err != nil {
		return nil, nil, fmt.Errorf("error executing accounts select SQL: %v: %w", err, rdb.ErrQuery)
	}
	defer rowsResult.Close()

	accounts := make([]AccountRow, 0)
	for rowsResult.Next() {
		var account AccountRow
		var balance string
		var accountNumber uint64
		var sequenceNumer uint64
		if err = rowsResult.Scan(
			&account.Address,
			&account.Type,
			&account.MaybeName,
			&account.MaybePubkey,
			&accountNumber,
			&sequenceNumer,
			&balance,
		); err != nil {
			if errors.Is(err, rdb.ErrNoRows) {
				return nil, nil, rdb.ErrNoRows
			}
			return nil, nil, fmt.Errorf("error scanning account row: %v: %w", err, rdb.ErrQuery)
		}
		account.AccountNumber = strconv.FormatUint(accountNumber, 10)
		account.SequenceNumber = strconv.FormatUint(sequenceNumer, 10)
		json.MustUnmarshalFromString(balance, &account.Balance)
		accounts = append(accounts, account)
	}

	paginationResult, err := rDbPagination.Result()
	if err != nil {
		return nil, nil, fmt.Errorf("error preparing pagination result: %v", err)
	}

	return accounts, paginationResult, nil
}

type AccountRow struct {
	Address        string     `json:"address"`
	Type           string     `json:"type"`
	MaybeName      *string    `json:"name"`
	MaybePubkey    *string    `json:"pubkey"`
	AccountNumber  string     `json:"accountNumber"`
	SequenceNumber string     `json:"sequenceNumber"`
	Balance        coin.Coins `json:"balance"`
}
