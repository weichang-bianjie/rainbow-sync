package block

import (
	"encoding/json"
	"fmt"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/irisnet/rainbow-sync/constant"
	model "github.com/irisnet/rainbow-sync/db"
	"github.com/irisnet/rainbow-sync/helper"
	"github.com/irisnet/rainbow-sync/logger"
	cmodel "github.com/irisnet/rainbow-sync/model"
	imsg "github.com/irisnet/rainbow-sync/model/msg"
	cutils "github.com/irisnet/rainbow-sync/utils"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/types"
	"gopkg.in/mgo.v2/bson"
	"gopkg.in/mgo.v2/txn"
	"time"
	"github.com/irisnet/rainbow-sync/conf"
	"strings"
	"strconv"
)

type ZoneBlock struct {
}

func (zone *ZoneBlock) Name() string {
	return conf.ZoneName
}

func (zone *ZoneBlock) SaveDocsWithTxn(blockDoc *cmodel.Block, cosmosTxs []cmodel.ZoneTx, taskDoc cmodel.SyncZoneTask) error {
	var (
		ops, cosmosTxsOps []txn.Op
	)

	if blockDoc.Height == 0 {
		return fmt.Errorf("invalid block, height equal 0")
	}
	blockDoc.Id = bson.NewObjectId()

	blockOp := txn.Op{
		C:      blockModel.Name(),
		Id:     bson.NewObjectId(),
		Insert: blockDoc,
	}

	if length := len(cosmosTxs); length > 0 {

		cosmosTxsOps = make([]txn.Op, 0, length)
		for _, v := range cosmosTxs {
			v.Id = bson.NewObjectId()
			op := txn.Op{
				C:      txModel.Name(),
				Id:     bson.NewObjectId(),
				Insert: v,
			}
			cosmosTxsOps = append(cosmosTxsOps, op)
		}
	}

	updateOp := txn.Op{
		C:      taskModel.Name(),
		Id:     taskDoc.ID,
		Assert: txn.DocExists,
		Update: bson.M{
			"$set": bson.M{
				"current_height":   taskDoc.CurrentHeight,
				"status":           taskDoc.Status,
				"last_update_time": taskDoc.LastUpdateTime,
			},
		},
	}

	ops = make([]txn.Op, 0, len(cosmosTxsOps)+2)
	ops = append(append(ops, blockOp), cosmosTxsOps...)
	if taskDoc.ID != "" {
		ops = append(ops, updateOp)
	}

	if len(ops) > 0 {
		err := model.Txn(ops)
		if err != nil {
			return err
		}
	}

	return nil
}

func (zone *ZoneBlock) ParseBlock(b int64, client *helper.RpcClient) (resBlock *cmodel.Block, cosmosTxs []cmodel.ZoneTx, resErr error) {

	defer func() {
		if err := recover(); err != nil {
			logger.Error("parse zone block fail", logger.Int64("height", b),
				logger.Any("err", err), logger.String("Chain Block", zone.Name()))

			resBlock = &cmodel.Block{}
			cosmosTxs = nil
			resErr = fmt.Errorf("%v", err)
		}
	}()

	cosmosTxsdata, err := zone.ParseZoneTxs(b, client)
	if err != nil {
		logger.Error("parse zone asset error", logger.String("error", err.Error()),
			logger.String("Chain Block", zone.Name()))
	}

	resBlock = &cmodel.Block{
		Height:     b,
		CreateTime: time.Now().Unix(),
	}
	cosmosTxs = cosmosTxsdata
	resErr = err
	return
}

// parse zone txs  from block result txs
func (zone *ZoneBlock) ParseZoneTxs(b int64, client *helper.RpcClient) ([]cmodel.ZoneTx, error) {
	resblock, err := client.Block(&b)
	if err != nil {
		logger.Warn("get block result err, now try again", logger.String("err", err.Error()),
			logger.String("Chain Block", zone.Name()))
		// there is possible parse block fail when in iterator
		var err2 error
		client2 := helper.GetTendermintClient()
		resblock, err2 = client2.Block(&b)
		client2.Release()
		if err2 != nil {
			return nil, err2
		}
	}

	//fmt.Printf("======>>resblock.Block.Txs:%+v\n",resblock.Block.Txs)
	//fmt.Println("length:",len(resblock.Block.Txs))

	cosmosTxs := make([]cmodel.ZoneTx, 0, len(resblock.Block.Txs))
	for _, tx := range resblock.Block.Txs {
		if cosmostx := zone.ParseZoneTxModel(tx, resblock.Block); len(cosmostx) > 0 {
			cosmosTxs = append(cosmosTxs, cosmostx...)
		}
	}

	return cosmosTxs, nil
}

func (zone *ZoneBlock) ParseZoneTxModel(txBytes types.Tx, block *types.Block) []cmodel.ZoneTx {
	var (
		authTx     auth.StdTx
		methodName = "parseZoneTxModel"
		txdetail   cmodel.ZoneTx
		docTxMsgs  []cmodel.DocTxMsg
	)

	cdc := cutils.GetCodec()
	err := cdc.UnmarshalBinaryBare(txBytes, &authTx)
	if err != nil {
		logger.Error(err.Error())
		return nil
	}
	status, result, err := QueryTxResult(txBytes.Hash())
	if err != nil {
		logger.Error("get txResult err", logger.String("method", methodName),
			logger.String("err", err.Error()),
			logger.String("Chain Block", zone.Name()))
	}
	//msgStat, err := parseRawlog(result.Log)
	//if err != nil {
	//	logger.Error("get parseRawlog err", logger.String("method", methodName),
	//		logger.String("err", err.Error()),
	//		logger.String("Chain Block", zone.Name()))
	//}

	fee := cutils.BuildFee(authTx.Fee)
	txdetail.TxHash = cutils.BuildHex(txBytes.Hash())
	txdetail.Height = block.Height
	txdetail.Memo = authTx.Memo
	txdetail.Fee = &fee
	txdetail.Time = block.Time
	txdetail.Status = status
	txdetail.Code = result.Code

	//length_msgStat := len(msgStat)

	msgs := authTx.GetMsgs()
	lenMsgs := len(msgs)
	if lenMsgs <= 0 {
		logger.Error("can't get msgs", logger.String("method", methodName),
			logger.String("Chain Block", zone.Name()))
		return nil
	}
	txs := make([]cmodel.ZoneTx, 0, lenMsgs)
	for _, msg := range msgs {
		txdetail.Initiator = ""
		txdetail.From = ""
		txdetail.To = ""
		txdetail.Amount = nil
		txdetail.Type = ""
		//if length_msgStat > i {
		//	txdetail.Status = msgStat[i]
		//}
		switch msg.(type) {

		case cmodel.MsgTransfer:
			txdetail.Events = parseEvents(result)
			msg := msg.(cmodel.MsgTransfer)
			txdetail.Initiator = msg.FromAddress.String()
			txdetail.From = msg.FromAddress.String()
			txdetail.To = msg.ToAddress.String()
			txdetail.Amount = cutils.ParseCoins(msg.Amount)
			txdetail.Type = constant.TxTypeTransfer

			docTxMsg := imsg.DocTxMsgTransfer{}
			docTxMsg.BuildMsg(msg)
			txdetail.Msgs = append(docTxMsgs, cmodel.DocTxMsg{
				Type: docTxMsg.Type(),
				Msg:  &docTxMsg,
			})
			txs = append(txs, txdetail)
			break

		case cmodel.IBCBankMsgTransfer:
			txdetail.Events = parseEvents(result)
			packethash, dstport, dstchannel := buildIBCPacketHashByEvents(txdetail.Events)
			msg := msg.(cmodel.IBCBankMsgTransfer)
			txdetail.Initiator = msg.Sender.String()
			txdetail.From = txdetail.Initiator
			txdetail.To = msg.Receiver.String()
			txdetail.Amount = cutils.ParseCoins(msg.Amount)
			txdetail.Type = constant.TxTypeIBCBankTransfer
			txdetail.IBCPacketHash = packethash
			txMsg := imsg.DocTxMsgIBCBankTransfer{}
			txMsg.BuildMsg(msg)
			txdetail.Msgs = append(docTxMsgs, cmodel.DocTxMsg{
				Type: txMsg.Type(),
				Msg:  &txMsg,
			})
			if len(txdetail.Amount) > 0 {
				if denom, ok := denomPrefixHandle(txdetail.Type, txdetail.Amount[0].Denom, dstport, dstchannel); ok {
					txdetail.Amount[0].Denom = denom
				}
			}
			txs = append(txs, txdetail)
			break
		case cmodel.IBCPacket:
			msg := msg.(cmodel.IBCPacket)
			txdetail.Initiator = msg.Signer.String()
			txdetail.Type = constant.TxTypeIBCBankMsgPacket
			txMsg := imsg.DocTxMsgIBCMsgPacket{}
			txMsg.BuildMsg(msg)
			txdetail.Msgs = append(docTxMsgs, cmodel.DocTxMsg{
				Type: txMsg.Type(),
				Msg:  &txMsg,
			})
			txdetail.From = txMsg.Packet.Data.Value.Sender
			txdetail.To = txMsg.Packet.Data.Value.Receiver
			txdetail.Amount = []*cmodel.Coin{}

			if len(txMsg.Packet.Data.Value.Amount) > 0 {
				amount := txMsg.Packet.Data.Value.Amount[0]
				coinval := cutils.ParseRewards(amount.Amount + amount.Denom)
				denom, ok := denomPrefixHandle(txdetail.Type, coinval.Denom, txMsg.Packet.DestinationPort, txMsg.Packet.DestinationChannel)
				if ok {
					coinval.Denom = denom
				}
				txdetail.Amount = append(txdetail.Amount, coinval)
			}
			packetBytes, _ := json.Marshal(txMsg.Packet)
			txdetail.IBCPacketHash = cutils.Md5Encrypt([]byte(string(packetBytes)))
			txs = append(txs, txdetail)
			break
		case cmodel.IBCTimeout:
			msg := msg.(cmodel.IBCTimeout)
			txdetail.Initiator = msg.Signer.String()
			txdetail.From = txdetail.Initiator
			txdetail.To = ""
			txdetail.Type = constant.TxTypeIBCBankMsgTimeout
			txMsg := imsg.DocTxMsgIBCTimeout{}
			txMsg.BuildMsg(msg)
			txdetail.Msgs = append(docTxMsgs, cmodel.DocTxMsg{
				Type: txMsg.Type(),
				Msg:  &txMsg,
			})
			txs = append(txs, txdetail)
			break
		case cmodel.MsgAddLiquidity:
			msg := msg.(cmodel.MsgAddLiquidity)
			coin := cutils.ParseCoin(msg.MaxToken)

			txdetail.From = msg.Sender.String()
			txdetail.To = ""
			txdetail.Amount = []*cmodel.Coin{{Denom: coin.Denom, Amount: coin.Amount}}
			txdetail.Type = constant.TxTypeAddLiquidity
			txMsg := imsg.DocTxMsgAddLiquidity{}
			txMsg.BuildMsg(msg)
			txdetail.Msgs = append(docTxMsgs, cmodel.DocTxMsg{
				Type: txMsg.Type(),
				Msg:  &txMsg,
			})
			txs = append(txs, txdetail)
			break
		case cmodel.MsgRemoveLiquidity:
			msg := msg.(cmodel.MsgRemoveLiquidity)
			coin := cutils.ParseCoin(msg.WithdrawLiquidity)

			txdetail.From = msg.Sender.String()
			txdetail.To = ""
			txdetail.Amount = []*cmodel.Coin{{Denom: coin.Denom, Amount: coin.Amount}}
			txdetail.Type = constant.TxTypeRemoveLiquidity
			txMsg := imsg.DocTxMsgRemoveLiquidity{}
			txMsg.BuildMsg(msg)
			txdetail.Msgs = append(docTxMsgs, cmodel.DocTxMsg{
				Type: txMsg.Type(),
				Msg:  &txMsg,
			})
			txs = append(txs, txdetail)
			break
		case cmodel.MsgSwapOrder:
			msg := msg.(cmodel.MsgSwapOrder)

			txdetail.From = msg.Input.Address.String()
			txdetail.To = msg.Output.Address.String()
			txdetail.Type = constant.TxTypeSwapOrder
			txMsg := imsg.DocTxMsgSwapOrder{}
			txMsg.BuildMsg(msg)
			txdetail.Msgs = append(docTxMsgs, cmodel.DocTxMsg{
				Type: txMsg.Type(),
				Msg:  &txMsg,
			})
			txdetail.Amount = swapOrderAmountHandle(txMsg, result, txdetail.Status)
			txs = append(txs, txdetail)
			break

		default:
			logger.Warn("unknown msg type", logger.String("msgtype", msg.Type()))
		}
	}

	return txs
}

func swapOrderAmountHandle(ordermsg imsg.DocTxMsgSwapOrder, result *abci.ResponseDeliverTx, status string) cmodel.Coins {
	var ret cmodel.Coins
	ret = append(ret, &cmodel.Coin{Denom: ordermsg.Input.Coin.Denom, Amount: ordermsg.Input.Coin.Amount})
	if status == constant.TxStatusSuccess {
		for _, e := range result.GetEvents() {
			if e.Type == constant.EventTypeCoinSwapTransfer {
				coin := &cmodel.Coin{}
				for _, attr := range e.Attributes {
					if string(attr.Key) != "amount" {
						continue
					}
					data := string(attr.Value)
					if index := strings.Index(data, ordermsg.Output.Coin.Denom); index > 0 {
						amountstr := string(attr.Value[:index])
						amount, _ := strconv.ParseFloat(amountstr, 64)
						coin.Denom = ordermsg.Output.Coin.Denom
						coin.Amount = amount
						break
					}
				}
				if coin.Denom == "" {
					continue
				}
				ret = append(ret, coin)
				break
			}
		}

	} else {
		ret = append(ret, &cmodel.Coin{Denom: ordermsg.Output.Coin.Denom, Amount: ordermsg.Output.Coin.Amount})
	}
	return ret
}

func denomPrefixHandle(txType, denom, port, channel string) (string, bool) {
	dstPrefix := port + "/" + channel
	segments := strings.Split(denom, "/")
	if len(segments) < 2 {
		return "", false
	}
	amountFPrefix := segments[0] + "/" + segments[1]
	switch txType {
	case constant.TxTypeIBCBankTransfer:
		if amountFPrefix == dstPrefix {
			return string([]byte(denom)[len(amountFPrefix)+1:]), true
		}
	case constant.TxTypeIBCBankMsgPacket:
		if amountFPrefix != dstPrefix {
			return string([]byte(denom)[len(amountFPrefix)+1:]), true
		}
	}
	return "", false
}

// get tx status and log by query txHash
func QueryTxResult(txHash []byte) (string, *abci.ResponseDeliverTx, error) {
	status := constant.TxStatusSuccess

	client := helper.GetTendermintClient()
	defer client.Release()

	res, err := client.Tx(txHash, false)
	if err != nil {
		return "unknown", nil, err
	}
	result := res.TxResult
	if result.Code != 0 {
		status = constant.TxStatusFail
	}

	return status, &result, nil
}

func parseEvents(result *abci.ResponseDeliverTx) []cmodel.Event {

	var events []cmodel.Event
	for _, val := range result.GetEvents() {
		if val.Type != constant.EventTypeSendPacket {
			continue
		}
		one := cmodel.Event{
			Type: val.Type,
		}
		one.Attributes = make(map[string]string, len(val.Attributes))
		for _, attr := range val.Attributes {
			one.Attributes[string(attr.Key)] = string(attr.Value)
		}
		events = append(events, one)
	}

	return events
}

//func buildCoins(denom string, amountStr string) []*cmodel.Coin {
//	var coins []*cmodel.Coin
//	amount, err := strconv.ParseFloat(amountStr, 64)
//	if err != nil {
//		logger.Error("convert str to float64 fail", logger.String("amountStr", amountStr),
//			logger.String("err", err.Error()))
//		amount = 0
//	}
//	coin := cmodel.Coin{
//		Denom:  denom,
//		Amount: amount,
//	}
//	return append(coins, &coin)
//}

func buildIBCPacketHashByEvents(events []cmodel.Event) (string, string, string) {
	var (
		packetStr      string
		packetSequence uint64
		dstPort        string
		dstChannel     string
		srcPort        string
		srcChannel     string
		packetTimeout  uint64
	)
	if len(events) == 0 {
		return "", dstPort, dstChannel
	}

	for _, e := range events {
		if e.Type == constant.EventTypeSendPacket {
			for k, v := range e.Attributes {
				if k == constant.EventAttributesKeyPacket {
					packetStr = v
				}
				if k == constant.EventAttributesKeySequence {
					packetSequence, _ = strconv.ParseUint(v, 10, 64)
					//packetSequence = v
				}
				if k == constant.EventAttributesKeyDstPort {
					dstPort = v
				}
				if k == constant.EventAttributesKeyDstChannel {
					dstChannel = v
				}
				if k == constant.EventAttributesKeySrcPort {
					srcPort = v
				}
				if k == constant.EventAttributesKeySrcChannel {
					srcChannel = v
				}
				if k == constant.EventAttributesKeyTimeout {
					packetTimeout, _ = strconv.ParseUint(v, 10, 64)
				}
				//if packetStr != "" && packetSequence > 0 && packetTimeout > 0 &&
				//	dstPort != "" && dstChannel != "" &&
				//	srcPort != "" && srcChannel != "" {
				//	break
				//}
			}
			break
		}
	}

	if packetStr == "" {
		return "", dstPort, dstChannel
	}

	var sendpacket imsg.SendPacket
	json.Unmarshal([]byte(packetStr), &sendpacket)

	packet := imsg.Packet{
		Sequence:           uint64(packetSequence),
		TimeoutHeight:      uint64(packetTimeout),
		SourcePort:         srcPort,
		SourceChannel:      srcChannel,
		DestinationPort:    dstPort,
		DestinationChannel: dstChannel,
		Data:               sendpacket,
	}

	packetBytes, _ := json.Marshal(packet)

	return cutils.Md5Encrypt(packetBytes), dstPort, dstChannel
}

//
//func buildIBCPacketHashByPacket(packet cmodel.IBCPacket) (string, error) {
//	data, err := packet.MarshalJSON()
//	if err != nil {
//		return "", err
//	}
//	return cutils.Md5Encrypt(data), nil
//}
//
//func buildIBCPacketData(packetData []byte) (cmodel.IBCTransferPacketDataValue, error) {
//	var transferPacketData cmodel.IBCTransferPacketData
//	err := json.Unmarshal(packetData, &transferPacketData)
//	if err != nil {
//		return transferPacketData.Value, err
//	}
//
//	return transferPacketData.Value, nil
//}

func parseRawlog(rawlog string) (map[int]string, error) {

	var Stats []cmodel.RawLog
	if err := json.Unmarshal([]byte(rawlog), &Stats); err != nil {
		return nil, err
	}

	msgStat := make(map[int]string, len(Stats))
	for _, stat := range Stats {
		if stat.Success {
			msgStat[stat.MsgIndex] = constant.TxStatusSuccess
		} else {
			msgStat[stat.MsgIndex] = constant.TxStatusFail
		}

	}
	return msgStat, nil
}
