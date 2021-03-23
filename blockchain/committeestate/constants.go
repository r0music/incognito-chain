package committeestate

var (
	MAX_SWAP_OR_ASSIGN_PERCENT = 6
	MAX_SLASH_PERCENT          = 3
	MAX_SWAP_OUT_PERCENT       = 8
	MAX_SWAP_IN_PERCENT        = 8
	MAX_ASSIGN_PERCENT         = 8
)

const (
	STATE_TEST_VERSION      = 0
	SELF_SWAP_SHARD_VERSION = 1
	SLASHING_VERSION        = 2
	DCS_VERSION             = 3
)

const (
	swapRuleTestVersion     = 0
	swapRuleSlashingVersion = 2
	swapRuleDCSVersion      = 3
)

const (
	unstakeRuleTestVersion     = 0
	unstakeRuleSlashingVersion = 1
	unstakeRuleDCSVersion      = 2
)

const (
	syncTerm            = 17280  //2 days with block time = 10s
	committeeTerm       = 259200 //30 days with block time = 10s
	MaxSubsetCommittees = 2
)
