package view

import (
	"github.com/AstraProtocol/astra-indexing/appinterface/pagination"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/stretchr/testify/mock"
)

type MockAccountsView struct {
	mock.Mock
}

func NewMockAccountsView(_ *rdb.Handle) Accounts {
	return &MockAccountsView{}
}

func (accountsView *MockAccountsView) Upsert(account *AccountRow) error {
	mockArgs := accountsView.Called(account)
	return mockArgs.Error(0)
}

func (accountsView *MockAccountsView) TotalAccount() (int64, error) {
	mockArgs := accountsView.Called()
	result, _ := mockArgs.Get(0).(int64)
	return result, mockArgs.Error(1)
}

func (accountsView *MockAccountsView) FindBy(identity *AccountIdentity) (*AccountRow, error) {
	mockArgs := accountsView.Called(identity)
	result, _ := mockArgs.Get(0).(*AccountRow)
	return result, mockArgs.Error(1)
}

func (accountsView *MockAccountsView) List(
	order AccountsListOrder,
	paginate *pagination.Pagination,
) ([]AccountRow, *pagination.Result, error) {
	mockArgs := accountsView.Called(order, paginate)
	result0, _ := mockArgs.Get(0).([]AccountRow)
	result1, _ := mockArgs.Get(1).(*pagination.Result)
	return result0, result1, mockArgs.Error(2)
}
