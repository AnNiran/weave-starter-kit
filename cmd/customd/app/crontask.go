package customd

import (
	"github.com/iov-one/weave"
	"github.com/iov-one/weave-starter-kit/x/custom"
	"github.com/iov-one/weave/errors"
	"github.com/iov-one/weave/x/countdown"
)

// CronTaskMarshaler is a task marshaler implementation to be used by the weave
// applications when dealing with scheduled tasks.
//
// This implementation relies on the CronTask protobuf declaration.
var CronTaskMarshaler = taskMarshaler{}

type taskMarshaler struct{}

// MarshalTask implements cron.TaskMarshaler interface.
func (taskMarshaler) MarshalTask(auth []weave.Condition, msg weave.Msg) ([]byte, error) {
	t := CronTask{
		Authenticators: auth,
	}

	switch msg := msg.(type) {
	default:
		return nil, errors.Wrapf(errors.ErrType, "unsupported message type: %T", msg)

	case *custom.DeleteTimedStateMsg:
		t.Sum = &CronTask_CustomDeleteTimedStateMsg{
			CustomDeleteTimedStateMsg: msg,
		}

	case *countdown.ResetMsg:
		t.Sum = &CronTask_CountdownResetMsg{
			CountdownResetMsg: msg,
		}
	case *countdown.LineMsg:
		t.Sum = &CronTask_CountdownLineMsg{
			CountdownLineMsg: msg,
		}
	}

	raw, err := t.Marshal()
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal")
	}
	return raw, nil
}

// UnmarshalTask implements cron.TaskMarshaler interface.
func (taskMarshaler) UnmarshalTask(raw []byte) ([]weave.Condition, weave.Msg, error) {
	var t CronTask
	if err := t.Unmarshal(raw); err != nil {
		return nil, nil, errors.Wrap(err, "cannot unmarshal")
	}
	msg, err := weave.ExtractMsgFromSum(t.GetSum())
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot extract message")
	}
	return t.Authenticators, msg, nil
}
