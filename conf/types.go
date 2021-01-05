package conf

import (
	"github.com/irisnet/rainbow-sync/logger"
	"github.com/irisnet/rainbow-sync/utils"
	"os"
	"strconv"
	"strings"
)

var (
	SvrConf               *ServerConf
	blockChainMonitorUrl  = []string{"tcp://192.168.150.31:16657"}
	chainBlockResetHeight = 0

	initConnectionNum = 50  // fast init num of tendermint client pool
	maxConnectionNum  = 100 // max size of tendermint client pool
)

type ServerConf struct {
	NodeUrls              []string
	ChainBlockResetHeight int64

	MaxConnectionNum  int
	InitConnectionNum int
}

const (
	EnvNameDbAddr     = "DB_ADDR"
	EnvNameDbUser     = "DB_USER"
	EnvNameDbPassWd   = "DB_PASSWD"
	EnvNameDbDataBase = "DB_DATABASE"

	EnvNameSerNetworkFullNodes   = "SER_BC_FULL_NODES"
	EnvNameChainBlockResetHeight = "CHAIN_BLOCK_RESET_HEIGHT"
)

// get value of env var
func init() {
	var err error

	nodeUrl, found := os.LookupEnv(EnvNameSerNetworkFullNodes)
	if found {
		blockChainMonitorUrl = strings.Split(nodeUrl, ",")
	}

	if v, found := os.LookupEnv(EnvNameChainBlockResetHeight); found {
		chainBlockResetHeight, err = strconv.Atoi(v)
		if err != nil {
			logger.Fatal("Can't convert str to int", logger.String(EnvNameChainBlockResetHeight, v))
		}
	}

	SvrConf = &ServerConf{
		NodeUrls:              blockChainMonitorUrl,
		ChainBlockResetHeight: int64(chainBlockResetHeight),

		MaxConnectionNum:  maxConnectionNum,
		InitConnectionNum: initConnectionNum,
	}
	logger.Debug("print server config", logger.String("serverConf", utils.MarshalJsonIgnoreErr(SvrConf)))
}
