// Copyright 2015 The go-ethereum Authors
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

package rdb

import (
	r "gopkg.in/gorethink/gorethink.v3"
	"github.com/ethereum/go-ethereum/core/types"
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/common"
	"gopkg.in/urfave/cli.v1"
	"crypto/x509"
	"os"
	"crypto/tls"
	"net/url"
	"sync"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"time"
)

var (
	EthVMFlag = cli.BoolFlag{
		Name:  "ethvm",
		Usage: "Save blockchain data to external db, default rethinkdb local",
	}
	EthVMRemoteFlag = cli.BoolFlag{
		Name:  "ethvm.remote",
		Usage: "use remote rethink database, make sure to set RETHINKDB_URL env variable ",
	}
	EthVMCertFlag = cli.BoolFlag{
		Name:  "ethvm.cert",
		Usage: "use custom ssl cert for rethinkdb connection, make sure to set RETHINKDB_CERT env variable ",
	}
	ctx       *cli.Context
	rUrl      string
	session   *r.Session
	DB_NAME   = "eth_mainnet"
	DB_Tables = map[string]string{
		"blocks":       "blocks",
		"transactions": "transactions",
		"traces":       "traces",
		"logs":         "logs",
	}
	TRACE_STR = "{transfers:[],isError:false,msg:'',result:function(){var _this=this;return{transfers:_this.transfers,isError:_this.isError,msg:_this.msg}},step:function(log,db){var _this=this;if(log.err){_this.isError=true;_this.msg=log.err.Error();return}var op=log.op;var stack=log.stack;var memory=log.memory;var transfer={};var from=log.account;if(op.toString()=='CALL'){transfer={op:'CALL',value:stack.peek(2).Bytes(),from:from,fromBalance:db.getBalance(from).Bytes(),to:big.BigToAddress(stack.peek(1)),toBalance:db.getBalance(big.BigToAddress(stack.peek(1))).Bytes(),input:memory.slice(big.ToInt(stack.peek(3)),big.ToInt(stack.peek(3))+big.ToInt(stack.peek(4)))};_this.transfers.push(transfer)}else if(op.toString()=='SELFDESTRUCT'){transfer={op:'SELFDESTRUCT',value:db.getBalance(from).Bytes(),from:from,fromBalance:db.getBalance(from).Bytes(),to:big.BigToAddress(stack.peek(0)),toBalance:db.getBalance(big.BigToAddress(stack.peek(0))).Bytes()};_this.transfers.push(transfer)}else if(op.toString()=='CREATE'){transfer={op:'CREATE',value:stack.peek(0).Bytes(),from:from,fromBalance:db.getBalance(from).Bytes(),to:big.CreateContractAddress(from,db.getNonce(from)),toBalance:db.getBalance(big.CreateContractAddress(from,db.getNonce(from))).Bytes()input:memory.slice(big.ToInt(stack.peek(1)),big.ToInt(stack.peek(1))+big.ToInt(stack.peek(2)))};_this.transfers.push(transfer)}}}"
)

type TxBlock struct {
	Tx        *types.Transaction
	Trace     interface{}
	Pending   bool
	Timestamp *big.Int
}
type BlockIn struct {
	Block           *types.Block
	TxBlocks        *[]TxBlock
	State           *state.StateDB
	PrevTd          *big.Int
	Receipts        types.Receipts
	Signer          types.Signer
	IsUncle         bool
	TxFees          *big.Int
	BlockRewardFunc func(block *types.Block) (*big.Int, *big.Int)
	UncleRewardFunc func(uncles []*types.Header, index int) *big.Int
	UncleReward     *big.Int
}

func Connect() error {
	var _session *r.Session
	var _err error
	if ctx.GlobalBool(EthVMFlag.Name) && !ctx.GlobalBool(EthVMRemoteFlag.Name) {
		_session, _err = r.Connect(r.ConnectOpts{
			Address: "localhost:28015",
		})
	} else if ctx.GlobalBool(EthVMRemoteFlag.Name) && !ctx.GlobalBool(EthVMCertFlag.Name) {
		rethinkurl, _ := url.Parse(os.Getenv("RETHINKDB_URL"))
		password, setpass := rethinkurl.User.Password()
		if !setpass {
			panic("Password needs to be set in $RETHINKDB_URL")
		}
		_session, _err = r.Connect(r.ConnectOpts{
			Address:  rethinkurl.Host,
			Username: rethinkurl.User.Username(),
			Password: password,
		})
	} else if ctx.GlobalBool(EthVMRemoteFlag.Name) && ctx.GlobalBool(EthVMCertFlag.Name) {
		roots := x509.NewCertPool()
		cert := os.Getenv("RETHINKDB_CERT_RAW")
		roots.AppendCertsFromPEM([]byte(cert))
		rethinkurl, _ := url.Parse(os.Getenv("RETHINKDB_URL"))
		password, setpass := rethinkurl.User.Password()
		if !setpass {
			panic("Password needs to be set in $RETHINKDB_URL")
		}
		if cert!= ""{
			_session, _err = r.Connect(r.ConnectOpts{
				Address:  rethinkurl.Host,
				Username: rethinkurl.User.Username(),
				Password: password,
				TLSConfig: &tls.Config{
					RootCAs: roots,
				},
			})
		} else {
			_session, _err = r.Connect(r.ConnectOpts{
				Address:  rethinkurl.Host,
				Username: rethinkurl.User.Username(),
				Password: password,
			})
		}

	}
	if _err == nil {
		session = _session
	} else {
		panic(_err)
	}
	r.DBCreate(DB_NAME).RunWrite(session)
	for _, v := range DB_Tables {
		r.DB(DB_NAME).TableCreate(v, r.TableCreateOpts{PrimaryKey: "hash"}).RunWrite(session)
	}
	r.DB(DB_NAME).Table(DB_Tables["transactions"]).IndexCreate("nonceHash").RunWrite(session)
	r.DB(DB_NAME).Table(DB_Tables["blocks"]).IndexCreate("intNumber").RunWrite(session)
	r.DB(DB_NAME).Table(DB_Tables["traces"]).IndexCreateFunc("trace_from", r.Row.Field("trace").Field("transfers").Field("from"), r.IndexCreateOpts{Multi: true}).RunWrite(session)
	r.DB(DB_NAME).Table(DB_Tables["traces"]).IndexCreateFunc("trace_to", r.Row.Field("trace").Field("transfers").Field("to"), r.IndexCreateOpts{Multi: true}).RunWrite(session)
	return _err
}
func InsertGenesis(gAlloc map[common.Address][]byte, block *types.Block) {
	if !ctx.GlobalBool(EthVMFlag.Name) {
		return
	}
	rTrace := map[string]interface{}{
		"hash":           common.BytesToHash([]byte("GENESIS_TX")).Bytes(),
		"blockHash":      block.Hash().Bytes(),
		"blockNumber":    block.Header().Number.Bytes(),
		"blockIntNumber": hexutil.Uint64(block.Header().Number.Uint64()),
		"trace": map[string]interface{}{
			"isError": false,
			"msg":     "",
			"transfers": func() interface{} {
				var dTraces []interface{}
				for addr, balance := range gAlloc {
					dTraces = append(dTraces, map[string]interface{}{
						"op":    "BLOCK",
						"value": balance,
						"to":    addr.Bytes(),
						"type":  "GENESIS",
					})
				}
				dTraces = append(dTraces, map[string]interface{}{
					"op":          "BLOCK",
					"txFees":      big.NewInt(0).Bytes(),
					"blockReward": big.NewInt(5e+18).Bytes(),
					"uncleReward": big.NewInt(0).Bytes(),
					"to":          common.BytesToAddress(make([]byte, 1)).Bytes(),
					"type":        "REWARD",
				})
				return dTraces
			}(),
		},
	}
	_, err := r.DB(DB_NAME).Table(DB_Tables["traces"]).Insert(rTrace, r.InsertOpts{
		Conflict: "replace",
	}).RunWrite(session)
	if err != nil {
		panic(err)
	}
}

type IPendingTx struct {
	Tx      *types.Transaction
	Trace   interface{}
	State   *state.StateDB
	Signer  types.Signer
	Receipt *types.Receipt
	Block   *types.Block
}
func IsDB() bool {
	return ctx.GlobalBool(EthVMFlag.Name)
}
func AddPendingTxs(pTxs []*IPendingTx) {
	var wg sync.WaitGroup
	if !ctx.GlobalBool(EthVMFlag.Name) {
		return
	}
	ts := big.NewInt(time.Now().Unix())
	var(
		txs []interface{}
		logs []interface{}
		traces []interface{}
	)
	for _, pTx := range pTxs {
		var tReceipts types.Receipts
		txBlock := TxBlock{
			Tx:        pTx.Tx,
			Trace:     pTx.Trace,
			Pending:   true,
			Timestamp: ts,
		}
		var tBlockIn = &BlockIn{
			Receipts: append(tReceipts, pTx.Receipt),
			Block:    pTx.Block,
			State:    pTx.State,
			Signer:   pTx.Signer,
		}
		_tTx, _tLogs, _tTrace := formatTx(tBlockIn, txBlock, 0)
		if _tTx!=nil {
			txs = append(txs, _tTx)
		}
		if _tLogs!=nil {
			logs = append(logs, _tLogs)
		}
		if(_tTrace != nil) {
			traces = append(traces, _tTrace)
		}
	}
	saveToDB := func(table string, values interface{}) {
		defer wg.Done()
		if values != nil {
			_, err := r.DB(DB_NAME).Table(DB_Tables[table]).Insert(values, r.InsertOpts{
				Conflict: func(id r.Term, oldDoc r.Term, newDoc r.Term) interface{} {
					return oldDoc
				}}).RunWrite(session)
			if err != nil {
				panic(err)
			}
		}
	}
	wg.Add(3)
	go saveToDB("transactions", txs)
	go saveToDB("logs", logs)
	go saveToDB("traces", traces)
	wg.Wait()
	//fmt.Printf("New Pending Txs %d \n", len(pTxs))

}
func formatTx(blockIn *BlockIn, txBlock TxBlock, index int) (interface{}, map[string]interface{}, map[string]interface{}) {
	tx := txBlock.Tx
	receipt := blockIn.Receipts[index]
	head := blockIn.Block.Header()
	if receipt == nil {
		log.Debug("Receipt not found for transaction", "hash", tx.Hash())
		return nil, nil, nil
	}
	signer := blockIn.Signer
	from, _ := types.Sender(signer, tx)
	_v, _r, _s := tx.RawSignatureValues()
	var fromBalance = blockIn.State.GetBalance(from)
	var toBalance = big.NewInt(0)
	if tx.To() != nil {
		toBalance = blockIn.State.GetBalance(*tx.To())
	}
	formatTopics := func(topics []common.Hash) ([][]byte) {
		arrTopics := make([][]byte, len(topics))
		for i, topic := range topics {
			arrTopics[i] = topic.Bytes()
		}
		return arrTopics
	}
	formatLogs := func(logs []*types.Log) (interface{}) {
		dLogs := make([]interface{}, len(logs))
		for i, log := range logs {
			logFields := map[string]interface{}{
				"address":     log.Address.Bytes(),
				"topics":      formatTopics(log.Topics),
				"data":        log.Data,
				"blockNumber": big.NewInt(int64(log.BlockNumber)).Bytes(),
				"txHash":      log.TxHash.Bytes(),
				"txIndex":     big.NewInt(int64(log.TxIndex)).Bytes(),
				"blockHash":   log.BlockHash.Bytes(),
				"index":       big.NewInt(int64(log.Index)).Bytes(),
				"removed":     log.Removed,
			}
			dLogs[i] = logFields
		}
		return dLogs
	}
	rfields := map[string]interface{}{
		"root":             blockIn.Block.Header().ReceiptHash.Bytes(),
		"blockHash":        blockIn.Block.Hash().Bytes(),
		"blockNumber":      head.Number.Bytes(),
		"blockIntNumber":   hexutil.Uint64(head.Number.Uint64()),
		"transactionIndex": big.NewInt(int64(index)).Bytes(),
		"from":             from.Bytes(),
		"fromBalance":      fromBalance.Bytes(),
		"to": func() []byte {
			if tx.To() == nil {
				return common.BytesToAddress(make([]byte, 1)).Bytes()
			} else {
				return tx.To().Bytes()
			}
		}(),
		"toBalance":         toBalance.Bytes(),
		"gasUsed":           receipt.GasUsed.Bytes(),
		"cumulativeGasUsed": receipt.CumulativeGasUsed.Bytes(),
		"contractAddress":   nil,
		"logsBloom":         receipt.Bloom.Bytes(),
		"gas":               tx.Gas().Bytes(),
		"gasPrice":          tx.GasPrice().Bytes(),
		"hash":              tx.Hash().Bytes(),
		"nonceHash":         crypto.Keccak256Hash(from.Bytes(), big.NewInt(int64(tx.Nonce())).Bytes()).Bytes(),
		"replacedBy":        make([]byte, 0),
		"input":             tx.Data(),
		"nonce":             big.NewInt(int64(tx.Nonce())).Bytes(),
		"value":             tx.Value().Bytes(),
		"v":                 (_v).Bytes(),
		"r":                 (_r).Bytes(),
		"s":                 (_s).Bytes(),
		"status":            receipt.Status,
		"pending":           txBlock.Pending,
		"timestamp":         txBlock.Timestamp.Bytes(),
	}
	rlogs := map[string]interface{}{
		"hash":           tx.Hash().Bytes(),
		"blockHash":      blockIn.Block.Hash().Bytes(),
		"blockNumber":    head.Number.Bytes(),
		"blockIntNumber": hexutil.Uint64(head.Number.Uint64()),
		"logs":           formatLogs(receipt.Logs),
	}
	getTxTransfer := func() []map[string]interface{} {
		var dTraces []map[string]interface{}
		dTraces = append(dTraces, map[string]interface{}{
			"op":    "TX",
			"from":  rfields["from"],
			"to":    rfields["to"],
			"value": rfields["value"],
			"input": rfields["input"],
		})
		return dTraces
	}
	rTrace := map[string]interface{}{
		"hash":           tx.Hash().Bytes(),
		"blockHash":      blockIn.Block.Hash().Bytes(),
		"blockNumber":    head.Number.Bytes(),
		"blockIntNumber": hexutil.Uint64(head.Number.Uint64()),
		"trace": func() interface{} {
			temp, ok := txBlock.Trace.(map[string]interface{})
			if !ok {
				temp = map[string]interface{}{
					"isError": true,
					"msg":     txBlock.Trace,
				}
			}
			isError := temp["isError"].(bool)
			transfers, ok := temp["transfers"].([]map[string]interface{})
			if (!isError && !ok) {
				temp["transfers"] = getTxTransfer()
			} else {
				temp["transfers"] = append(transfers, getTxTransfer()[0])
			}
			return temp
		}(),
	}
	if len(receipt.Logs) == 0 {
		rlogs["logs"] = nil
		rfields["logsBloom"] = nil
	}
	// If the ContractAddress is 20 0x0 bytes, assume it is not a contract creation
	if receipt.ContractAddress != (common.Address{}) {
		rfields["contractAddress"] = receipt.ContractAddress
	}
	return rfields, rlogs, rTrace
}
func InsertBlock(blockIn *BlockIn) {
	if !ctx.GlobalBool(EthVMFlag.Name) {
		return
	}
	processTxs := func(txblocks *[]TxBlock) ([][]byte, []interface{}, []interface{}, []interface{}) {
		var tHashes [][]byte
		var tTxs []interface{}
		var tLogs []interface{}
		var tTrace []interface{}
		if txblocks == nil {
			return tHashes, tTxs, tLogs, tTrace
		}
		for i, _txBlock := range *txblocks {
			_tTx, _tLogs, _tTrace := formatTx(blockIn, _txBlock, i)
			tTxs = append(tTxs, _tTx)
			if (_tLogs["logs"] != nil) {
				tLogs = append(tLogs, _tLogs)
			}
			if (_tTrace["trace"] != nil) {
				tTrace = append(tTrace, _tTrace)
			}
			tHashes = append(tHashes, _txBlock.Tx.Hash().Bytes())
		}
		return tHashes, tTxs, tLogs, tTrace
	}
	formatBlock := func(block *types.Block, tHashes [][]byte) (map[string]interface{}, error) {
		head := block.Header() // copies the header once
		minerBalance := blockIn.State.GetBalance(head.Coinbase)
		txFees, blockReward, uncleReward := func() ([]byte, []byte, []byte) {
			var (
				_txfees []byte
				_uncleR []byte
				_blockR []byte
			)
			if blockIn.TxFees != nil {
				_txfees = blockIn.TxFees.Bytes()
			} else {
				_txfees = make([]byte, 0)
			}
			if blockIn.IsUncle {
				_blockR = blockIn.UncleReward.Bytes()
				_uncleR = make([]byte, 0)
			} else {
				blockR, uncleR := blockIn.BlockRewardFunc(block)
				_blockR, _uncleR = blockR.Bytes(), uncleR.Bytes()

			}
			return _txfees, _blockR, _uncleR
		}()
		bfields := map[string]interface{}{
			"number":       head.Number.Bytes(),
			"intNumber":    hexutil.Uint64(head.Number.Uint64()),
			"hash":         head.Hash().Bytes(),
			"parentHash":   head.ParentHash.Bytes(),
			"nonce":        head.Nonce,
			"mixHash":      head.MixDigest.Bytes(),
			"sha3Uncles":   head.UncleHash.Bytes(),
			"logsBloom":    head.Bloom.Bytes(),
			"stateRoot":    head.Root.Bytes(),
			"miner":        head.Coinbase.Bytes(),
			"minerBalance": minerBalance.Bytes(),
			"difficulty":   head.Difficulty.Bytes(),
			"totalDifficulty": func() []byte {
				if blockIn.PrevTd == nil {
					return make([]byte, 0)
				}
				return (new(big.Int).Add(block.Difficulty(), blockIn.PrevTd)).Bytes()
			}(),
			"extraData":         head.Extra,
			"size":              big.NewInt(block.Size().Int64()).Bytes(),
			"gasLimit":          head.GasLimit.Bytes(),
			"gasUsed":           head.GasUsed.Bytes(),
			"timestamp":         head.Time.Bytes(),
			"transactionsRoot":  head.TxHash.Bytes(),
			"receiptsRoot":      head.ReceiptHash.Bytes(),
			"transactionHashes": tHashes,
			"uncleHashes": func() [][]byte {
				uncles := make([][]byte, len(block.Uncles()))
				for i, uncle := range block.Uncles() {
					uncles[i] = uncle.Hash().Bytes()
					InsertBlock(&BlockIn{
						Block:       types.NewBlockWithHeader(uncle),
						State:       blockIn.State,
						IsUncle:     true,
						UncleReward: blockIn.UncleRewardFunc(block.Uncles(), i),
					})
					fmt.Printf("New Uncle block %s \n", uncle.Hash().String())
				}
				return uncles
			}(),
			"isUncle":     blockIn.IsUncle,
			"txFees":      txFees,
			"blockReward": blockReward,
			"uncleReward": uncleReward,
		}
		return bfields, nil
	}
	tHashes, tTxs, tLogs, tTrace := processTxs(blockIn.TxBlocks)
	block, _ := formatBlock(blockIn.Block, tHashes)
	if (block["intNumber"] != 0) {
		tTrace = append(tTrace, map[string]interface{}{
			"hash":           block["hash"],
			"blockHash":      block["hash"],
			"blockNumber":    block["number"],
			"blockIntNumber": block["intNumber"],
			"trace": map[string]interface{}{
				"isError": false,
				"msg":     "",
				"transfers": func() interface{} {
					var dTraces []interface{}
					dTraces = append(dTraces, map[string]interface{}{
						"op":          "BLOCK",
						"txFees":      block["txFees"],
						"blockReward": block["blockReward"],
						"uncleReward": block["uncleReward"],
						"to":          block["miner"],
						"type":        "REWARD",
					})
					return dTraces
				}(),
			},
		})
	}
	saveToDB := func() {
		var wg sync.WaitGroup
		wg.Add(3)
		saveToDB := func(table string, values interface{}, isWait bool) {
			if values != nil {
				_, err := r.DB(DB_NAME).Table(DB_Tables[table]).Insert(values, r.InsertOpts{
					Conflict: "replace",
				}).RunWrite(session)
				if err != nil {
					panic(err)
				}
			}
			if isWait {
				wg.Done()
			}
		}
		updateNonceHashes := func() {
			for _, tx := range tTxs {
				tx, ok := tx.(map[string]interface{})
				if !ok {
					panic(ok)
				}
				r.DB(DB_NAME).Table(DB_Tables["transactions"]).GetAllByIndex("nonceHash", tx["nonceHash"]).Update(map[string]interface{}{"replacedBy": tx["hash"], "pending": false, }).RunWrite(session)
			}
		}
		go updateNonceHashes()
		go saveToDB("transactions", tTxs, true)
		go saveToDB("logs", tLogs, true)
		go saveToDB("traces", tTrace, true)
		wg.Wait()
		saveToDB("blocks", block, false)
	}
	go saveToDB()
}

func NewRethinkDB(_ctx *cli.Context) {
	ctx = _ctx
	if ctx.GlobalBool(EthVMFlag.Name) {
		err := Connect()
		if err != nil {
			panic("couldnt connect to rethinkdb")
		}
	}
}
