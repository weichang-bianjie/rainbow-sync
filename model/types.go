package model

import "github.com/irisnet/rainbow-sync/db"

var (
	BlockModel Block
	TxModel    Tx

	Collections = []db.Docs{
		BlockModel,
		TxModel,
	}
)

func EnsureDocsIndexes() {
	if len(Collections) > 0 {
		for _, v := range Collections {
			v.EnsureIndexes()
		}
	}
}
