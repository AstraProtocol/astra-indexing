package event

import (
	"bytes"
	"github.com/AstraProtocol/astra-indexing/usecase/model"

	entity_event "github.com/AstraProtocol/astra-indexing/entity/event"
	jsoniter "github.com/json-iterator/go"
	"github.com/luci/go-render/render"
)

const MSG_CREATE_CRAW_BACK_VESTING_ACCOUNT = "/evmos.vesting.v1.MsgCreateClawbackVestingAccount"
const MSG_CREATE_CRAW_BACK_VESTING_ACCOUNT_CREATED = "/evmos.vesting.v1.MsgCreateClawbackVestingAccount.Created"
const MSG_CREATE_CRAW_BACK_VESTING_ACCOUNT_FAILED = "/evmos.vesting.v1.MsgCreateClawbackVestingAccount.Failed"

type MsgCreateClawbackVestingAccount struct {
	MsgBase
	Params model.MsgCreateClawbackVestingAccountParams `json:"params"`
}

func NewMsgCreateClawbackVestingAccount(msgCommonParams MsgCommonParams,
	params model.MsgCreateClawbackVestingAccountParams) *MsgCreateClawbackVestingAccount {
	return &MsgCreateClawbackVestingAccount{
		NewMsgBase(MsgBaseParams{
			MsgName:         MSG_CREATE_CRAW_BACK_VESTING_ACCOUNT,
			Version:         1,
			MsgCommonParams: msgCommonParams,
		}),
		params,
	}
}

// ToJSON encodes the event into JSON string payload
func (event *MsgCreateClawbackVestingAccount) ToJSON() (string, error) {
	encoded, err := jsoniter.Marshal(event)
	if err != nil {
		return "", err
	}

	return string(encoded), nil
}

func (event *MsgCreateClawbackVestingAccount) String() string {
	return render.Render(event)
}

func DecodeMsgCreateClawbackVestingAccount(encoded []byte) (entity_event.Event, error) {
	jsonDecoder := jsoniter.NewDecoder(bytes.NewReader(encoded))
	jsonDecoder.DisallowUnknownFields()

	var event *MsgCreateClawbackVestingAccount
	if err := jsonDecoder.Decode(&event); err != nil {
		return nil, err
	}

	return event, nil
}
