package ppos

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/ninjadotorg/cash-prototype/cashec"
	"github.com/ninjadotorg/cash-prototype/common"
	"github.com/ninjadotorg/cash-prototype/common/base58"
	"github.com/ninjadotorg/cash-prototype/mempool"

	peer2 "github.com/libp2p/go-libp2p-peer"
	"github.com/ninjadotorg/cash-prototype/blockchain"
	"github.com/ninjadotorg/cash-prototype/wire"
)

// PoSEngine only need to start if node runner want to be a validator

type Engine struct {
	sync.Mutex
	started bool
	wg      sync.WaitGroup
	quit    chan struct{}

	sealerStarted bool

	// channel
	cQuitSealer chan struct{}
	cBlockSig   chan blockSig

	config                EngineConfig
	currentCommittee      []string
	candidates            []string
	knownChainsHeight     chainsHeight
	validatedChainsHeight chainsHeight
}

type ChainInfo struct {
	CurrentCommittee  []string
	CandidateListHash string
	ChainsHeight      []int
}
type chainsHeight struct {
	Heights []int
	sync.Mutex
}

type EngineConfig struct {
	BlockChain *blockchain.BlockChain
	// RewardAgent
	ChainParams     *blockchain.Params
	BlockGen        *blockchain.BlkTmplGenerator
	MemPool         *mempool.TxPool
	ValidatorKeySet cashec.KeySetSealer
	Server          interface {
		// list functions callback which are assigned from Server struct
		GetPeerIDsFromPublicKey(string) []peer2.ID
		PushMessageToAll(wire.Message) error
		PushMessageToPeer(wire.Message, peer2.ID) error
		PushMessageGetChainState() error
	}
	FeeEstimator map[byte]*mempool.FeeEstimator
}

type blockSig struct {
	BlockHash    string
	Validator    string
	ValidatorSig string
}

//Start start consensus engine
func (self *Engine) Start() error {
	self.Lock()
	defer self.Unlock()
	if self.started {
		self.Unlock()
		return errors.New("Consensus engine is already started")
	}
	Logger.log.Info("Starting Parallel Proof of Stake Consensus engine")
	self.knownChainsHeight.Heights = make([]int, common.TotalValidators)
	self.validatedChainsHeight.Heights = make([]int, common.TotalValidators)
	self.currentCommittee = make([]string, 20)

	for chainID := 0; chainID < common.TotalValidators; chainID++ {
		self.knownChainsHeight.Heights[chainID] = int(self.config.BlockChain.BestState[chainID].Height)
	}

	Logger.log.Info("Validating local blockchain...")
	for chainID := 0; chainID < common.TotalValidators; chainID++ {
		//Don't validate genesis block (blockHeight = 1)
		for blockHeight := 2; blockHeight < self.knownChainsHeight.Heights[chainID]; blockHeight++ {
			block, err := self.config.BlockChain.GetBlockByBlockHeight(int32(blockHeight), byte(chainID))
			if err != nil {
				Logger.log.Error(err)
				return err
			}
			err = self.validateBlockSanity(block)
			if err != nil {
				Logger.log.Error(err)
				return err
			}
		}
	}

	copy(self.validatedChainsHeight.Heights, self.knownChainsHeight.Heights)
	copy(self.currentCommittee, self.config.BlockChain.BestState[0].BestBlock.Header.Committee)

	go func() {
		for {
			self.config.Server.PushMessageGetChainState()
			time.Sleep(common.GetChainStateInterval * time.Second)
		}
	}()

	self.started = true
	self.quit = make(chan struct{})
	self.wg.Add(1)

	return nil
}

//Stop stop consensus engine
func (self *Engine) Stop() error {
	Logger.log.Info("Stopping Consensus engine...")
	self.Lock()
	defer self.Unlock()

	if !self.started {
		return errors.New("Consensus engine isn't running")
	}
	self.StopSealer()
	close(self.quit)
	self.started = false
	Logger.log.Info("Consensus engine stopped")
	return nil
}

//Init apply configuration to consensus engine
func (self Engine) Init(cfg *EngineConfig) (*Engine, error) {
	return &Engine{
		config: *cfg,
	}, nil
}

//StartSealer start sealing block
func (self *Engine) StartSealer(sealerKeySet cashec.KeySetSealer) {
	if self.sealerStarted {
		Logger.log.Error("Sealer already started")
		return
	}
	self.config.ValidatorKeySet = sealerKeySet

	self.cQuitSealer = make(chan struct{})
	self.cBlockSig = make(chan blockSig)
	self.sealerStarted = true
	Logger.log.Info("Starting sealer with public key: " + base58.Base58Check{}.Encode(self.config.ValidatorKeySet.SpublicKey, byte(0x00)))

	go func() {
		for {
			select {
			case <-self.cQuitSealer:
				return
			default:
				if self.started {
					if common.IntArrayEquals(self.knownChainsHeight.Heights, self.validatedChainsHeight.Heights) {
						chainID := self.getMyChain()
						if chainID < common.TotalValidators {
							Logger.log.Info("(๑•̀ㅂ•́)و Yay!! It's my turn")
							Logger.log.Info("Current chainsHeight")
							Logger.log.Info(self.validatedChainsHeight.Heights)
							Logger.log.Info("My chainID: ", chainID)

							newBlock, err := self.createBlock()
							if err != nil {
								Logger.log.Error(err)
								continue
							}
							err = self.Finalize(newBlock)
							if err != nil {
								Logger.log.Critical(err)
								continue
							}
						}
					} else {
						for i, v := range self.knownChainsHeight.Heights {
							if v > self.validatedChainsHeight.Heights[i] {
								lastBlockHash := self.config.BlockChain.BestState[i].BestBlockHash.String()
								getBlkMsg := &wire.MessageGetBlocks{
									LastBlockHash: lastBlockHash,
								}
								self.config.Server.PushMessageToAll(getBlkMsg)
							}
						}
					}
				}
			}
		}
	}()
}

// StopSealer stop sealer
func (self *Engine) StopSealer() {
	if self.sealerStarted {
		Logger.log.Info("Stopping Sealer...")
		close(self.cQuitSealer)
		close(self.cBlockSig)
		self.sealerStarted = false
	}
}

func (self *Engine) createBlock() (*blockchain.Block, error) {
	Logger.log.Info("Start creating block...")
	myChainID := self.getMyChain()
	paymentAddress, err := self.config.ValidatorKeySet.GetPaymentAddress()
	newblock, err := self.config.BlockGen.NewBlockTemplate(paymentAddress, myChainID)
	if err != nil {
		return &blockchain.Block{}, err
	}
	newblock.Block.Header.ChainsHeight = make([]int, common.TotalValidators)
	copy(newblock.Block.Header.ChainsHeight, self.validatedChainsHeight.Heights)
	newblock.Block.Header.ChainID = myChainID
	newblock.Block.ChainLeader = base58.Base58Check{}.Encode(self.config.ValidatorKeySet.SpublicKey, byte(0x00))

	copy(newblock.Block.Header.Committee, self.GetNextCommittee())

	sig, err := self.signData([]byte(newblock.Block.Hash().String()))
	if err != nil {
		return &blockchain.Block{}, err
	}
	newblock.Block.Header.BlockCommitteeSigs[newblock.Block.Header.ChainID] = sig
	return newblock.Block, nil
}

// Finalize after successfully create a block we will send this block to other validators to get their signatures
func (self *Engine) Finalize(block *blockchain.Block) error {
	Logger.log.Info("Start finalizing block...")
	finalBlock := block
	allSigReceived := make(chan struct{})
	cancel := make(chan struct{})
	committee := block.Header.Committee
	defer func() {
		close(cancel)
		close(allSigReceived)
	}()

	// Collect signatures of other validators
	go func(blockHash string) {
		var sigsReceived int
		for {
			select {
			case <-cancel:
				return
			case blocksig := <-self.cBlockSig:
				Logger.log.Info("Validator's signature received", sigsReceived)

				if blockHash != blocksig.BlockHash {
					Logger.log.Critical("Block hash not match!", blocksig, "this block", blockHash)
					continue
				}

				if idx := common.IndexOfStr(blocksig.Validator, committee); idx != -1 {
					if block.Header.BlockCommitteeSigs[idx] != "" {
						err := cashec.ValidateDataB58(blocksig.Validator, blocksig.ValidatorSig, []byte(block.Hash().String()))

						if err != nil {
							Logger.log.Error("Validate sig error:", err)
							continue
						} else {
							sigsReceived++
							finalBlock.Header.BlockCommitteeSigs[idx] = blocksig.ValidatorSig
						}
					} else {
						Logger.log.Error("Already received this validator blocksig")
					}
				}

				if sigsReceived == (common.MinBlockSigs - 1) {
					allSigReceived <- struct{}{}
					return
				}
			}
		}
	}(block.Hash().String())
	//Request for signatures of other validators
	go func() {
		reqSigMsg, _ := wire.MakeEmptyMessage(wire.CmdRequestSign)
		reqSigMsg.(*wire.MessageRequestSign).Block = *block
		for idx := 0; idx < common.TotalValidators; idx++ {
			//@TODO: retry on failed validators
			if committee[idx] != block.ChainLeader {
				go func(validator string) {
					peerIDs := self.config.Server.GetPeerIDsFromPublicKey(validator)
					if len(peerIDs) != 0 {
						Logger.log.Info("Request signaure from "+peerIDs[0], validator)
						self.config.Server.PushMessageToPeer(reqSigMsg, peerIDs[0])
					} else {
						Logger.log.Error("Validator's peer not found!", validator)
					}
				}(committee[idx])
			}
		}
	}()

	// Wait for signatures of other validators
	select {
	case <-allSigReceived:
		Logger.log.Info("Validator sigs: ", finalBlock.Header.BlockCommitteeSigs)
	case <-time.After(common.MaxBlockSigWaitTime * time.Second):
		return errCantFinalizeBlock
	}

	headerBytes, _ := json.Marshal(finalBlock.Header)
	sig, err := self.signData(headerBytes)
	if err != nil {
		return err
	}
	finalBlock.ChainLeaderSig = sig
	self.UpdateChain(finalBlock)
	self.sendBlockMsg(block)
	return nil
}

func (self *Engine) UpdateChain(block *blockchain.Block) {
	// save block into fee estimator
	err := self.config.FeeEstimator[block.Header.ChainID].RegisterBlock(block)
	if err != nil {
		Logger.log.Error(err)
	}
	// update tx pool
	for _, tx := range block.Transactions {
		self.config.MemPool.RemoveTx(tx)
	}

	self.config.BlockChain.BestState[block.Header.ChainID].Update(block)

	self.knownChainsHeight.Lock()
	if self.knownChainsHeight.Heights[block.Header.ChainID] < int(block.Height) {
		self.knownChainsHeight.Heights[block.Header.ChainID] = int(block.Height)
		self.sendBlockMsg(block)
	}
	self.knownChainsHeight.Unlock()
	self.validatedChainsHeight.Lock()
	self.validatedChainsHeight.Heights[block.Header.ChainID] = int(block.Height)
	self.validatedChainsHeight.Unlock()
}
