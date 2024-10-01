package Addrman

import (
	"encoding/json"
	"log"
	"sync"

	"go.etcd.io/bbolt"
)

type Addrman struct {
	db   *bbolt.DB
	rwMu sync.RWMutex
}

var once sync.Once
var instance *Addrman

func GetAddrMan(path string) *Addrman {
	once.Do(func() {
		db, err := bbolt.Open(path, 0666, nil)
		if err != nil {
			log.Fatal(err)
		}
		instance = &Addrman{
			db:   db,
			rwMu: sync.RWMutex{},
		}
	})
	return instance
}

func (am *Addrman) InIt(path string) {
	GetAddrMan(path)
}

func (am *Addrman) Serialize(data string) ([]byte, error) {
	return json.Marshal(data)
}

func (am *Addrman) Deserialize(data []byte) (string, error) {
	var info string
	err := json.Unmarshal(data, &info)
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	return info, nil
}

func (am *Addrman) SaveAddr(addr string, info string) (bool, error) {
	err := am.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("Peers"))
		if err != nil {
			log.Fatal(err)
		}
		data, err := am.Serialize(info)
		if err != nil {
			log.Fatal(err)
		}
		key := []byte(addr)
		return bucket.Put(key, data)
	})
	return err == nil, err
}

func (am *Addrman) GetAddr() map[string]string {
	peers := make(map[string]string)
	am.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("Peers"))
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(k, v []byte) error {
			info, err := am.Deserialize(v)
			if err != nil {
				return err
			}
			peers[string(k)] = info
			return nil
		})
	})
	return peers
}

func (am *Addrman) DoesAddrAlreadyExist(key string) bool {
	var exist bool
	am.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("Peers"))
		if bucket == nil {
			return nil
		}
		value := bucket.Get([]byte(key))
		exist = value != nil
		return nil
	})
	return exist
}

func (am *Addrman) Close() {
	am.db.Close()
}
