package command

//
//import (
//	entity_event "github.com/AstraProtocol/astra-indexing/entity/event"
//	"github.com/AstraProtocol/astra-indexing/usecase/event"
//	ibc_model "github.com/AstraProtocol/astra-indexing/usecase/model/ibc"
//)
//
//type CreateMsgUpdateClient struct {
//	msgCommonParams event.MsgCommonParams
//	params          ibc_model.MsgUpdateClientParams
//}
//
//func NewCreateMsgUpdateClient(
//	msgCommonParams event.MsgCommonParams,
//	params ibc_model.MsgUpdateClientParams,
//) *CreateMsgUpdateClient {
//	return &CreateMsgUpdateClient{
//		msgCommonParams,
//		params,
//	}
//}
//
//func (*CreateMsgUpdateClient) Name() string {
//	return "CreateMsgUpdateClient"
//}
//
//func (*CreateMsgUpdateClient) Version() int {
//	return 1
//}
//
//func (cmd *CreateMsgUpdateClient) Exec() (entity_event.Event, error) {
//	event := event.NewMsgUpdateClient(cmd.msgCommonParams, cmd.params)
//	return event, nil
//}
