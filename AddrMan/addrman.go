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

// will return a singleton
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
func (am *Addrman) Serialize(data []string) ([]byte, error) {
	return json.Marshal(data)
}
func (am *Addrman) DeSerialize(data []byte) ([]string, error) {
	var info []string
	err := json.Unmarshal(data, &info)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	return info, nil
}
func (am *Addrman) SaveAddr(addr string, info []string) (bool, error) {
	am.db.Update(func(tx *bbolt.Tx) error {
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
	return true, nil
}
func (am *Addrman) GetAddr() map[string][]string {
	peers := make(map[string][]string)

	am.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("Peers"))
		if bucket == nil {
			log.Fatal("Bucket Doesnt exist")
		}
		return bucket.ForEach(func(k, v []byte) error {
			info, err := am.DeSerialize(v)
			if err != nil {
				log.Fatal(err)
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
