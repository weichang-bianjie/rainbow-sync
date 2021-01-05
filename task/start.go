package task

import (
	"context"
	"github.com/irisnet/rainbow-sync/block"
	irisConf "github.com/irisnet/rainbow-sync/conf"
	"github.com/irisnet/rainbow-sync/lib/pool"
	"github.com/irisnet/rainbow-sync/logger"
	models "github.com/irisnet/rainbow-sync/model"
	"gopkg.in/mgo.v2"
	"time"
)

func Start() {
	pool.Init(irisConf.SvrConf.NodeUrls, irisConf.SvrConf.MaxConnectionNum, irisConf.SvrConf.InitConnectionNum)
	SyncTx := func() {

		heightChan := make(chan int64)
		go getHeightTask(heightChan)

		for {
			select {
			case height, ok := <-heightChan:
				if ok {
					parseBlockAndSave(height)
				}

			}

		}
	}

	go SyncTx()

	//go new(cron.CronService).StartCronService()
}

func getHeightTask(chanHeight chan int64) {
	inProcessBlock := int64(1)
	if maxHeight, err := new(models.Block).GetMaxHeight(); err != nil {
		if err != mgo.ErrNotFound {
			logger.Fatal("get max height in block table have error",
				logger.String("err", err.Error()))
		}
	} else {
		inProcessBlock = maxHeight
	}
	if irisConf.SvrConf.ChainBlockResetHeight > 0 {
		inProcessBlock = irisConf.SvrConf.ChainBlockResetHeight
	}
	for {
		blockChainLatestHeight, err := getBlockChainLatestHeight()
		if err != nil {
			logger.Warn("get blockChain latest height err", logger.String("err", err.Error()))
		}
		if blockChainLatestHeight < inProcessBlock && blockChainLatestHeight > 0 {
			logger.Info("wait blockChain latest height update",
				logger.Int64("curSyncedHeight", inProcessBlock-1),
				logger.Int64("blockChainLatestHeight", blockChainLatestHeight))
			time.Sleep(1 * time.Second)
			continue
		}
		if blockChainLatestHeight >= inProcessBlock {
			chanHeight <- inProcessBlock
			inProcessBlock++
		}
	}
}

func parseBlockAndSave(height int64) {
	// parse data from block
	client := pool.GetClient()
	defer func() {
		client.Release()
	}()
	blockDoc, txDocs, err := block.ParseBlock(height, client)
	if err != nil {
		logger.Fatal("Parse block fail", logger.Int64("height", height),
			logger.String("err", err.Error()))
	}
	if err := block.SaveDocsWithTxn(blockDoc, txDocs); err != nil {
		logger.Fatal("save docs fail", logger.String("err", err.Error()))
	}
	logger.Info("sync blockChain have ok",
		logger.Int64("curSyncedHeight", height))
}

// get current block height
func getBlockChainLatestHeight() (int64, error) {
	client := pool.GetClient()
	defer func() {
		client.Release()
	}()
	status, err := client.Status(context.Background())
	if err != nil {
		return 0, err
	}

	return status.SyncInfo.LatestBlockHeight, nil
}
