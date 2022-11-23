package command

import (
	entity_event "github.com/AstraProtocol/astra-indexing/entity/event"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	"github.com/AstraProtocol/astra-indexing/usecase/model"
)

type CreateMsgEthereumTx struct {
	msgCommonParams event.MsgCommonParams
	params          model.MsgEthereumTxParams
}

func NewCreateMsgEthereumTx(
	msgCommonParams event.MsgCommonParams,
	params model.MsgEthereumTxParams,
) *CreateMsgEthereumTx {
	return &CreateMsgEthereumTx{
		msgCommonParams,
		params,
	}
}

func (*CreateMsgEthereumTx) Name() string {
	return "/ethermint.evm.v1.MsgEthereumTx.Create"
}

func (*CreateMsgEthereumTx) Version() int {
	return 1
}

func (cmd *CreateMsgEthereumTx) Exec() (entity_event.Event, error) {
	event := event.NewMsgEthereumTx(cmd.msgCommonParams, cmd.params)
	return event, nil
}
