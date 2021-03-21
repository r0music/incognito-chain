package blsbftv4

import (
	"github.com/incognitochain/incognito-chain/blockchain/types"
	signatureschemes2 "github.com/incognitochain/incognito-chain/consensus_v2/signatureschemes"
	"github.com/incognitochain/incognito-chain/incognitokey"
)

type ProposeBlockInfo struct {
	block                types.BlockInterface
	committees           []incognitokey.CommitteePublicKey
	committeesForSigning []incognitokey.CommitteePublicKey
	userKeySet           []signatureschemes2.MiningKey
	votes                map[string]*BFTVote //pk->BFTVote
	isValid              bool
	hasNewVote           bool
	isVoted              bool
	validVotes           int
	errVotes             int
}

//NewProposeBlockInfoValue : new propose block info
func newProposeBlockForProposeMsg(
	block types.BlockInterface,
	committees []incognitokey.CommitteePublicKey,
	committeesForSigning []incognitokey.CommitteePublicKey,
	userKeySet []signatureschemes2.MiningKey,
	votes map[string]*BFTVote,
	isValid, hasNewVote bool,
) *ProposeBlockInfo {
	return &ProposeBlockInfo{
		block:                block,
		committees:           incognitokey.DeepCopy(committees),
		committeesForSigning: incognitokey.DeepCopy(committeesForSigning),
		userKeySet:           signatureschemes2.DeepCopyMiningKeyArray(userKeySet),
		votes:                votes,
		isValid:              isValid,
		hasNewVote:           hasNewVote,
	}
}

func (proposeBlockInfo *ProposeBlockInfo) addBlockInfo(
	block types.BlockInterface,
	committees []incognitokey.CommitteePublicKey,
	committeesForSigning []incognitokey.CommitteePublicKey,
	userKeySet []signatureschemes2.MiningKey,
	validVotes, errVotes int,
) {
	proposeBlockInfo.block = block
	proposeBlockInfo.committees = incognitokey.DeepCopy(committees)
	proposeBlockInfo.committeesForSigning = incognitokey.DeepCopy(committeesForSigning)
	proposeBlockInfo.userKeySet = signatureschemes2.DeepCopyMiningKeyArray(userKeySet)
	proposeBlockInfo.validVotes = validVotes
	proposeBlockInfo.errVotes = errVotes
}

func newBlockInfoForVoteMsg() *ProposeBlockInfo {
	return &ProposeBlockInfo{
		votes:      make(map[string]*BFTVote),
		hasNewVote: true,
	}
}
