package command

import (
	entity_event "github.com/AstraProtocol/astra-indexing/entity/event"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	"github.com/AstraProtocol/astra-indexing/usecase/model"
)

type CreateMsgCreateValidator struct {
	msgCommonParams event.MsgCommonParams
	params          model.MsgCreateValidatorParams
}

func NewCreateMsgCreateValidator(msgCommonParams event.MsgCommonParams, params model.MsgCreateValidatorParams) *CreateMsgCreateValidator {
	return &CreateMsgCreateValidator{
		msgCommonParams,
		params,
	}
}

func (*CreateMsgCreateValidator) Name() string {
	return "/cosmos.staking.v1beta1.MsgCreateValidator.Create"
}

func (*CreateMsgCreateValidator) Version() int {
	return 1
}

func (cmd *CreateMsgCreateValidator) Exec() (entity_event.Event, error) {

	event := event.NewMsgCreateValidator(cmd.msgCommonParams, cmd.params)
	return event, nil
}
