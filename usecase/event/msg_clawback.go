package event

import (
	"bytes"

	"github.com/AstraProtocol/astra-indexing/usecase/model"

	entity_event "github.com/AstraProtocol/astra-indexing/entity/event"
	jsoniter "github.com/json-iterator/go"
	"github.com/luci/go-render/render"
)

const MSG_CLAW_BACK = "/evmos.vesting.v1.MsgClawback"
const MSG_CLAW_BACK_CREATED = "/evmos.vesting.v1.MsgClawback.Created"
const MSG_CLAW_BACK_FAILED = "/evmos.vesting.v1.MsgClawback.Failed"

type MsgClawback struct {
	MsgBase
	Params model.MsgClawbackParams `json:"params"`
}

func NewMsgClawback(msgCommonParams MsgCommonParams,
	params model.MsgClawbackParams) *MsgClawback {
	return &MsgClawback{
		NewMsgBase(MsgBaseParams{
			MsgName:         MSG_CLAW_BACK,
			Version:         1,
			MsgCommonParams: msgCommonParams,
		}),
		params,
	}
}

// ToJSON encodes the event into JSON string payload
func (event *MsgClawback) ToJSON() (string, error) {
	encoded, err := jsoniter.Marshal(event)
	if err != nil {
		return "", err
	}

	return string(encoded), nil
}

func (event *MsgClawback) String() string {
	return render.Render(event)
}

func DecodeMsgClawback(encoded []byte) (entity_event.Event, error) {
	jsonDecoder := jsoniter.NewDecoder(bytes.NewReader(encoded))
	jsonDecoder.DisallowUnknownFields()

	var event *MsgClawback
	if err := jsonDecoder.Decode(&event); err != nil {
		return nil, err
	}

	return event, nil
}
