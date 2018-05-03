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
	"github.com/wanchain/go-wanchain/common/hexutil"
	"math/big"
)

/*
 * This is a test memory database. Do not use for any production it does not get persisted
 */
type JSONDatabase struct {
	db   map[string][]byte
	lock sync.RWMutex
	count *big.Int
}

func NewJSONDatabase() (*JSONDatabase, error) {
	return &JSONDatabase{
		db: make(map[string][]byte),
		count: big.NewInt(0),
	}, nil
}

func (db *JSONDatabase) Put(key []byte, value []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	db.db[string(key)] = common.CopyBytes(value)
	db.count.Add(db.count,big.NewInt(1))
	return nil
}

func(db *JSONDatabase) GetDB() interface{} {
	statedb := make([]interface{}, db.count.Uint64())
	i := big.NewInt(0)
	for key := range db.db {
		state := map[string]interface{}{
			"key":     hexutil.Encode([]byte(key)),
			"value":   func()string {
				if entry, ok := db.db[string(key)]; ok {
					return hexutil.Encode(common.CopyBytes(entry))
				}
				return "0x0"
			}(),
		}
		statedb[i.Uint64()] = state
		i.Add(i,big.NewInt(1))
	}
	return statedb
}