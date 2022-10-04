package view

import (
	testify_mock "github.com/stretchr/testify/mock"

	pagination2 "github.com/AstraProtocol/astra-indexing/appinterface/pagination"
)

type MockIBCChannelMessageView struct {
	testify_mock.Mock
}

func (ibcChannelMessagesView *MockIBCChannelMessageView) Insert(ibcChannelMessage *IBCChannelMessageRow) error {
	mockArgs := ibcChannelMessagesView.Called(ibcChannelMessage)
	return mockArgs.Error(0)
}

func (ibcChannelMessagesView *MockIBCChannelMessageView) ListByChannelID(
	channelID string,
	order IBCChannelMessagesListOrder,
	filter IBCChannelMessagesListFilter,
	pagination *pagination2.Pagination,
) (
	[]IBCChannelMessageRow,
	*pagination2.Result,
	error,
) {
	mockArgs := ibcChannelMessagesView.Called(channelID, order, filter, pagination)
	messages, _ := mockArgs.Get(0).([]IBCChannelMessageRow)
	paginationResult, _ := mockArgs.Get(1).(*pagination2.Result)
	return messages, paginationResult, mockArgs.Error(3)
}
