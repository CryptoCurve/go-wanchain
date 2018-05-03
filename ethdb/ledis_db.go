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
	"sync"
	"github.com/wanchain/go-wanchain/common"
	lediscfg "github.com/siddontang/ledisdb/config"
	"github.com/siddontang/ledisdb/ledis"
)

/*
 * This is a test memory database. Do not use for any production it does not get persisted
 */
type LedisDatabase struct {
	db   *ledis.DB
	ledis *ledis.Ledis
	lock sync.RWMutex
}

func NewLedisDatabase() (*LedisDatabase, error) {
	cfg := lediscfg.NewConfigDefault()
	l, _ := ledis.Open(cfg)
	db, _ := l.Select(0)
	return &LedisDatabase{
		db: db,
		ledis:l,
	}, nil
}

func (db *LedisDatabase) Put(key []byte, value []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	err := db.db.Set(key, value)
	if err!=nil{
		panic(err)
	}
	return nil
}

func (db *LedisDatabase) Has(key []byte) (bool, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	ok,err := db.db.Get(key)

	return ok!=nil, err
}

func (db *LedisDatabase) Get(key []byte) ([]byte, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	dat, err := db.db.Get(key)
	return dat, err
}

func (db *LedisDatabase) Delete(key []byte) error {

	db.lock.Lock()
	defer db.lock.Unlock()

	db.db.Del(key)
	return nil
}

func (db *LedisDatabase) Close() {
	db.ledis.Close()
}

func (db *LedisDatabase) NewBatch() Batch {
	return &ledisBatch{db: db, size:0}
}

type ledisBatch struct {
	writes []kv
	size   int
	db   *LedisDatabase
}

func (b *ledisBatch) Put(key, value []byte) error {
	b.writes = append(b.writes, kv{common.CopyBytes(key), common.CopyBytes(value)})
	b.size += len(value)
	return nil
}

func (b *ledisBatch) Write() error {
	b.db.lock.Lock()
	defer b.db.lock.Unlock()

	for _, kv := range b.writes {
		b.db.db.Set(kv.k, kv.v)
	}
	return nil
}

func (b *ledisBatch) ValueSize() int {
	return b.size
}