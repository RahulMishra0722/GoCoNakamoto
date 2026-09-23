package MinerCacheMan

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/RahulMishra0722/GoCoNakamoto/DBMan"
	"go.etcd.io/bbolt"
)

const (
	dbName      = "./TempCache"
	bucketName  = "BlockTemplate"
	templateKey = "Template"
	dbFileMode  = 0600
)

type MinerCacheMan struct {
	db *bbolt.DB
}

func NewMinerCacheMan() (*MinerCacheMan, error) {
	db, err := bbolt.Open(dbName, dbFileMode, nil)
	if err != nil {
		return nil, fmt.Errorf("error opening DB: %w", err)
	}

	cm := &MinerCacheMan{db: db}

	err = cm.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return fmt.Errorf("error creating bucket %s: %w", bucketName, err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error initializing CacheMan: %w", err)
	}

	return cm, nil
}

func (cm *MinerCacheMan) SetBlockTemplate(template DBMan.BlockTemplate) error {
	return cm.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		if bucket == nil {
			return fmt.Errorf("bucket %s does not exist", bucketName)
		}

		var buff bytes.Buffer
		if err := gob.NewEncoder(&buff).Encode(template); err != nil {
			return fmt.Errorf("error encoding template: %w", err)
		}

		return bucket.Put([]byte(templateKey), buff.Bytes())
	})
}

func (cm *MinerCacheMan) GetBlockTemplate() (DBMan.BlockTemplate, error) {
	var blockTemplate DBMan.BlockTemplate

	err := cm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		if bucket == nil {
			return fmt.Errorf("bucket %s does not exist", bucketName)
		}

		data := bucket.Get([]byte(templateKey))
		if data == nil {
			return fmt.Errorf("no template found")
		}

		return gob.NewDecoder(bytes.NewReader(data)).Decode(&blockTemplate)
	})

	if err != nil {
		return DBMan.BlockTemplate{}, fmt.Errorf("error getting block template: %w", err)
	}

	return blockTemplate, nil
}

func (cm *MinerCacheMan) ClearDB() error {
	return cm.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		if bucket == nil {
			return fmt.Errorf("bucket %s does not exist", bucketName)
		}
		return bucket.Delete([]byte(templateKey))
	})
}

func (cm *MinerCacheMan) Close() error {
	return cm.db.Close()
}
