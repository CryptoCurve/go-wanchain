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
	"cloud.google.com/go/datastore"
	"golang.org/x/net/context"
	"fmt"
)

/*
 * This is a test memory database. Do not use for any production it does not get persisted
 */
type DataStore struct {
	db   *datastore.Client
	ctx context.Context
}

type DSValue struct {
	value    []byte    `datastore:"value"`
}

func NewDataStoreDB() (*DataStore, error) {
	ctx:= context.Background()
	dsClient, err := datastore.NewClient(ctx, "ethvm-189007")
	if err != nil {
		panic(err)
	}
	return &DataStore{
		db: dsClient,
		ctx:ctx,
	}, nil
}

func (db *DataStore) Put(key []byte, value []byte) error {
	_key := datastore.IncompleteKey(common.Bytes2Hex(key), nil)
	fmt.Printf("key %s \n",common.Bytes2Hex(key))
	_, err := db.db.Put(db.ctx,_key, &DSValue{
		value: value,
	})
	if err!=nil{
		panic(err)
	}
	return nil
}

func (db *DataStore) Has(key []byte) (bool, error) {
	_key := datastore.IncompleteKey(common.Bytes2Hex(key), nil)
	dsvalue := &DSValue{}
	err := db.db.Get(db.ctx, _key, dsvalue)
	return dsvalue.value!=nil, err
}

func (db *DataStore) Get(key []byte) ([]byte, error) {
	_key := datastore.IncompleteKey(common.Bytes2Hex(key), nil)
	dsvalue := &DSValue{}
	err := db.db.Get(db.ctx, _key, dsvalue)
	return dsvalue.value, err
}

func (db *DataStore) Delete(key []byte) error {
	_key := datastore.IncompleteKey(common.Bytes2Hex(key), nil)
	return  db.db.Delete(db.ctx,_key)
}

func (db *DataStore) Close() {
	db.db.Close()
}

func (db *DataStore) NewBatch() Batch {
	return &dataStoreBatch{db: db, size:0}
}

type dataStoreBatch struct {
	writes []kv
	size   int
	db   *DataStore
}

func (b *dataStoreBatch) Put(key, value []byte) error {
	b.writes = append(b.writes, kv{common.CopyBytes(key), common.CopyBytes(value)})
	b.size += len(value)
	return nil
}

func (b *dataStoreBatch) Write() error {

	var keys []*datastore.Key
	var values []*DSValue
	for _, kv := range b.writes {
		keys = append(keys, datastore.IncompleteKey(common.Bytes2Hex(kv.k), nil))
		values = append(values, &DSValue{value:kv.v})
	}
	_, err := b.db.db.PutMulti(b.db.ctx, keys, values)
	return err
}

func (b *dataStoreBatch) ValueSize() int {
	return b.size
}
