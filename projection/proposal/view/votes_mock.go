package view

import (
	testify_mock "github.com/stretchr/testify/mock"

	pagination2 "github.com/AstraProtocol/astra-indexing/appinterface/pagination"
)

type MockVotesView struct {
	testify_mock.Mock
}

func NewMockVotesView() Votes {
	return &MockVotesView{}
}

func (votesView *MockVotesView) Insert(row *VoteRow) error {
	mockArgs := votesView.Called(row)
	return mockArgs.Error(0)
}

func (votesView *MockVotesView) Update(row *VoteRow) error {
	mockArgs := votesView.Called(row)
	return mockArgs.Error(0)
}

func (votesView *MockVotesView) FindByProposalIdVoter(
	proposalId string,
	voterAddress string,
) (
	*VoteWithMonikerRow,
	error,
) {
	mockArgs := votesView.Called(proposalId, voterAddress)
	result1, _ := mockArgs.Get(0).(*VoteWithMonikerRow)
	return result1, mockArgs.Error(1)
}

func (votesView *MockVotesView) ListByProposalId(
	proposalId string,
	order VoteListOrder,
	filters Filters,
	pagination *pagination2.Pagination,
) (
	[]VoteWithMonikerRow,
	*pagination2.Result,
	error,
) {
	mockArgs := votesView.Called(proposalId, order, filters, pagination)
	result1, _ := mockArgs.Get(0).([]VoteWithMonikerRow)
	result2, _ := mockArgs.Get(1).(*pagination2.Result)
	return result1, result2, mockArgs.Error(2)
}
