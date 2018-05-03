// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package ethdb

import (
	"github.com/wanchain/go-wanchain/common"
	"github.com/go-redis/redis"
)

/*
 * This is a test memory database. Do not use for any production it does not get persisted
 */
type CacheDB struct {
	ethdb Database
	db   *redis.Client
}

func NewCacheDB(_ethdb Database) (*CacheDB, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	return &CacheDB{
		db: client,
		ethdb: _ethdb,
	}, nil
}

func (db *CacheDB) Put(key []byte, value []byte) error {
	return nil
}

func (db *CacheDB) Has(key []byte) (bool, error) {
	_, err := db.db.Get(string(key)).Result()
	if err == redis.Nil {
		return db.ethdb.Has(key)
	} else if err != nil {
		panic(err)
	}
	return err != redis.Nil, err
}

func (db *CacheDB) Get(key []byte) ([]byte, error)  {
	val, err := db.db.Get(string(key)).Result()
	if err == redis.Nil {
		savedVal, err := db.ethdb.Get(key)
		if err!=nil {
			return nil, err
		} else {
			db.db.Set(string(key),savedVal, 0).Err()
			return savedVal, nil
		}
	} else if err != nil {
		panic(err)
	} else {
		return []byte(val), err
	}
}

func (db *CacheDB) Delete(key []byte) error {
	//db.db.Del(key)
	return nil
}

func (db *CacheDB) Close() {
	db.db.Close()
}

func (db *CacheDB) NewBatch() Batch {
	return &cacheBatch{db: db, size:0}
}

type cacheBatch struct {
	writes []kv
	size   int
	db   *CacheDB
}

func (b *cacheBatch) Put(key, value []byte) error {
	b.writes = append(b.writes, kv{common.CopyBytes(key), common.CopyBytes(value)})
	b.size += len(value)
	return nil
}

func (b *cacheBatch) Write() error {
	/*for _, kv := range b.writes {
		b.db.db.Set(kv.k, kv.v)
	}*/
	return nil
}

func (b *cacheBatch) ValueSize() int {
	return b.size
}
