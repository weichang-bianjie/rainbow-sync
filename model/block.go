package model

import (
	"github.com/irisnet/rainbow-sync/db"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	CollectionNameBlock = "sync_iris_block"
)

type (
	Block struct {
		Height     int64 `bson:"height"`
		CreateTime int64 `bson:"create_time"`
	}
)

func (d Block) Name() string {
	return CollectionNameBlock
}

func (d Block) EnsureIndexes() {
	var indexes []mgo.Index
	indexes = append(indexes, mgo.Index{
		Key:        []string{"-height"},
		Unique:     true,
		Background: true,
	})
	db.EnsureIndexes(d.Name(), indexes)
}

func (d Block) PkKvPair() map[string]interface{} {
	return bson.M{"height": d.Height}
}
func (d Block) GetMaxHeight() (int64, error) {
	cond := bson.M{
		"height": bson.M{
			"$gt": 0,
		},
	}
	selecter := bson.M{
		"height": 1,
	}
	var block Block
	fn := func(c *mgo.Collection) error {
		return c.Find(cond).Sort("-height").Select(selecter).One(&block)
	}
	if err := db.ExecCollection(d.Name(), fn); err != nil {
		return 0, err
	}
	return block.Height, nil
}
