package txpool

import (
	"fmt"
	"runtime"
	"time"

	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/patrickmn/go-cache"
)

type TxInfo struct {
	Fee   uint64
	Size  uint64
	VTime time.Duration
}

type TxInfoDetail struct {
	Hash  string
	Fee   uint64
	Size  uint64
	VTime time.Duration
	Tx    metadata.Transaction
}

type TxsData struct {
	TxByHash map[string]metadata.Transaction
	TxInfos  map[string]TxInfo
}

type txInfoTemp struct {
	tx metadata.Transaction
	vt time.Duration
}

type TxsPool struct {
	// SID       byte
	action    chan func(*TxsPool)
	Verifier  TxVerifier
	Data      TxsData
	Cacher    *cache.Cache
	Inbox     chan metadata.Transaction
	isRunning bool
	cQuit     chan bool
	better    func(txA, txB metadata.Transaction) bool
}

func NewTxsPool(
	txVerifier TxVerifier,
	inbox chan metadata.Transaction,
) *TxsPool {
	return &TxsPool{
		action:    make(chan func(*TxsPool)),
		Verifier:  txVerifier,
		Data:      TxsData{},
		Cacher:    new(cache.Cache),
		Inbox:     inbox,
		isRunning: false,
		cQuit:     make(chan bool),
	}
}

func (tp *TxsPool) Start() {
	if tp.isRunning {
		return
	}
	tp.isRunning = true
	cValidTxs := make(chan txInfoTemp, 128)
	stopGetTxs := make(chan interface{})
	go tp.getTxs(stopGetTxs, cValidTxs)
	for {
		select {
		case <-tp.cQuit:
			stopGetTxs <- nil
			return
		case validTx := <-cValidTxs:
			txH := validTx.tx.Hash().String()
			tp.Data.TxByHash[txH] = validTx.tx
			tp.Data.TxInfos[txH] = TxInfo{
				Fee:   validTx.tx.GetTxFee(),
				Size:  validTx.tx.GetTxActualSize(),
				VTime: validTx.vt,
			}
		case f := <-tp.action:
			f(tp)
		}
	}
}

func (tp *TxsPool) Stop() {
	tp.cQuit <- true
}

func (tp *TxsPool) RemoveTxs(txHashes []string) {
	tp.action <- func(tpTemp *TxsPool) {
		for _, tx := range txHashes {
			delete(tpTemp.Data.TxByHash, tx)
			delete(tpTemp.Data.TxInfos, tx)
		}
	}
}

func (tp *TxsPool) ValidateNewTx(tx metadata.Transaction) (bool, error, time.Duration) {
	start := time.Now()
	if _, exist := tp.Cacher.Get(tx.Hash().String()); exist {
		return false, nil, 0
	}
	ok, err := tp.Verifier.ValidateWithoutChainstate(tx)
	return ok, err, time.Since(start)
}

func (tp *TxsPool) GetTxsTranferForNewBlock(
	cView metadata.ChainRetriever,
	sView metadata.ShardViewRetriever,
	bcView metadata.BeaconViewRetriever,
	maxSize uint64,
	maxTime time.Duration,
) []metadata.Transaction {
	res := []metadata.Transaction{}
	txDetailCh := make(chan *TxInfoDetail)
	stopCh := make(chan interface{})
	go tp.getTxsFromPool(txDetailCh, stopCh)
	curSize := uint64(0)
	curTime := 0 * time.Millisecond
	mapForChkDbSpend := map[[privacy.Ed25519KeySize]byte]struct {
		Index  uint
		Detail TxInfoDetail
	}{}
	for txDetails := range txDetailCh {
		if (curSize+txDetails.Size > maxSize) || (curTime+txDetails.VTime > maxTime) {
			continue
		}
		ok, err := tp.Verifier.ValidateWithChainState(
			txDetails.Tx,
			cView,
			sView,
			bcView,
			sView.GetBeaconHeight(),
		)
		if !ok || err != nil {
			fmt.Printf("Validate tx %v return error %v\n", txDetails.Hash, err)
		}
		ok, removedInfo := tp.CheckDoubleSpend(mapForChkDbSpend, txDetails.Tx, res)
		if ok {
			curSize = curSize - removedInfo.Fee + txDetails.Fee
			curTime = curTime - removedInfo.VTime + txDetails.VTime

		}
	}
	return res
}

func (tp *TxsPool) CheckDoubleSpend(
	dataHelper map[[privacy.Ed25519KeySize]byte]struct {
		Index  uint
		Detail TxInfoDetail
	},
	tx metadata.Transaction,
	txs []metadata.Transaction,
) (
	bool,
	TxInfo,
) {
	iCoins := tx.GetProof().GetInputCoins()
	oCoins := tx.GetProof().GetInputCoins()
	removedInfos := TxInfo{
		Fee:   0,
		VTime: 0,
	}
	removeIdx := map[uint]interface{}{}
	for _, iCoin := range iCoins {
		if info, ok := dataHelper[iCoin.CoinDetails.GetSerialNumber().ToBytes()]; ok {
			if _, ok := removeIdx[info.Index]; ok {
				continue
			}
			if tp.better(info.Detail.Tx, tx) {
				return false, removedInfos
			} else {
				removeIdx[info.Index] = nil
			}
		}
	}
	for _, oCoin := range oCoins {
		if info, ok := dataHelper[oCoin.CoinDetails.GetSNDerivator().ToBytes()]; ok {
			if _, ok := removeIdx[info.Index]; ok {
				continue
			}
			if tp.better(info.Detail.Tx, tx) {
				return false, removedInfos
			} else {
				removeIdx[info.Index] = nil
			}
		}
	}
	if len(removeIdx) > 0 {
		for k, v := range dataHelper {
			if _, ok := removeIdx[v.Index]; ok {
				delete(dataHelper, k)
			}
		}
		for k := range removeIdx {
			txs = append(txs[:k], txs[k+1:]...)
		}
	}

	return true, removedInfos
}

func insertTxIntoList(
	dataHelper map[[privacy.Ed25519KeySize]byte]struct {
		Index  uint
		Detail TxInfoDetail
	},
	txDetail TxInfoDetail,
	txs []metadata.Transaction,
) {
	tx := txDetail.Tx
	iCoins := tx.GetProof().GetInputCoins()
	oCoins := tx.GetProof().GetInputCoins()
	for _, iCoin := range iCoins {
		dataHelper[iCoin.CoinDetails.GetSerialNumber().ToBytes()] = struct {
			Index  uint
			Detail TxInfoDetail
		}{
			Index:  uint(len(txs)),
			Detail: txDetail,
		}
	}
	for _, oCoin := range oCoins {
		dataHelper[oCoin.CoinDetails.GetSNDerivator().ToBytes()] = struct {
			Index  uint
			Detail TxInfoDetail
		}{
			Index:  uint(len(txs)),
			Detail: txDetail,
		}
	}
	txs = append(txs, tx)
}

func (tp *TxsPool) CheckValidatedTxs(
	txs []metadata.Transaction,
) (
	valid []metadata.Transaction,
	needValidate []metadata.Transaction,
) {
	poolData := tp.snapshotPool()
	for _, tx := range txs {
		if _, ok := poolData.TxInfos[tx.Hash().String()]; ok {
			valid = append(valid, tx)
		} else {
			needValidate = append(needValidate, tx)
		}
	}
	return valid, needValidate
}

func (tp *TxsPool) getTxs(quit <-chan interface{}, cValidTxs chan txInfoTemp) {
	MAX := runtime.NumCPU() - 1
	nWorkers := make(chan int, MAX)
	for {
		select {
		case <-quit:
			return
		default:
			msg := <-tp.Inbox
			nWorkers <- 1
			go func() {
				isValid, err, vTime := tp.ValidateNewTx(msg)
				<-nWorkers
				if err != nil {
					fmt.Printf("Validate tx %v return error %v:\n", msg.Hash().String(), err)
				}
				if isValid {
					cValidTxs <- txInfoTemp{
						msg,
						vTime,
					}
				}
			}()
		}
	}
}

func (tp *TxsPool) snapshotPool() TxsData {
	cData := make(chan TxsData)
	tp.action <- func(tpTemp *TxsPool) {
		res := TxsData{
			TxByHash: map[string]metadata.Transaction{},
			TxInfos:  map[string]TxInfo{},
		}
		for k, v := range tpTemp.Data.TxByHash {
			res.TxByHash[k] = v
		}
		for k, v := range tpTemp.Data.TxInfos {
			res.TxInfos[k] = v
		}
		cData <- res
	}
	return <-cData
}

func (tp *TxsPool) getTxsFromPool(
	txCh chan *TxInfoDetail,
	stopC <-chan interface{},
) {
	tp.action <- func(tpTemp *TxsPool) {
		defer close(txCh)
		txDeTails := &TxInfoDetail{}
		for k, v := range tpTemp.Data.TxByHash {
			select {
			case <-stopC:
				return
			default:
				if info, ok := tpTemp.Data.TxInfos[k]; ok {
					txDeTails.Hash = k
					txDeTails.Fee = info.Fee
					txDeTails.Size = info.Size
					txDeTails.VTime = info.VTime
				} else {
					continue
				}
				if v != nil {
					txDeTails.Tx = v
					txCh <- txDeTails
				}
			}
		}
	}

}

// func (tp *TxsPool) removeTxs(tp)
