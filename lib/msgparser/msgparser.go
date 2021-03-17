package msgparser

import (
	msg_parser "github.com/weichang-bianjie/msg-sdk"
)

var (
	MsgClient msg_parser.MsgClient
)

func init() {
	MsgClient = msg_parser.NewMsgClient()
}
