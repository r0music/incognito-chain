package blockchain

// constant for network
const (
	MAINNET             = 0xd9b4bef9
	MAINNET_NAME        = "mainnet"
	MAINET_DEFAULT_PORT = "9333"
)

// constant for genesis block
const (
	GENESIS_BLOCK_REWARD = uint64(1000000000)
	//GENESIS_BLOCK_PUBKEY_ADDR  = "mgnUx4Ah4VBvtaL7U1VXkmRjKUk3h8pbst"
	GENESIS_BLOCK_PAYMENT_ADDR = "12Rt2dt1UT6PjZ7HzDVjkP5nAXZ22vPYWD1b31XuJ4FJBPorWpGbKTACT8wyfHqwDRqg3EuX2zAU9YQZvB6bNMNTsSGjqXHMQw9H1Xn"
	// readonly-key 13hVcoFvEFAnkGmmcHg6b54i6FbDEwP4cRDQHzCVP1NY6mkFnbk1ytK1V6cuNRHfL2Zz31ZxH4Aw8mwe9J5RgWVxLHFJMZxoV8JGyzo
	// pri-key 11111119q5P6bukedopEFUh7HDuiobEhcXb8VxdygNTzNoyDyXPzmAN13UXRKnwuXPEehA6AfD9UyGbsfKsg1aKvnf8AfX6nnfSQVr9bHio
)

// global variables for genesis blok
var (
	GENESIS_BLOCK_ANCHORS           = [][32]byte{[32]byte{1}, [32]byte{2}}
	GENESIS_BLOCK_NULLIFIERS        = []string{"88d35350b1846ecc34d6d04a10355ad9a8e1252e9d7f3af130186b4faf1a9832", "286b563fc45b7d5b9f929fb2c2766382a9126483d8d64c9b0197d049d4e89bf7"}
	GENESIS_BLOCK_COMMITMENTS       = []string{"d26356e6f726dfb4c0a395f3af134851139ce1c64cfed3becc3530c8c8ad5660", "5aaf71f995db014006d630dedf7ffcbfa8854055e6a8cc9ef153629e3045b7e1"}
	GENESIS_BLOCK_OUTPUT_R1         = [32]byte{1}
	GENESIS_BLOCK_OUTPUT_R2         = [32]byte{2}
	GENESIS_BLOCK_OUTPUT_R          = [][]byte{GENESIS_BLOCK_OUTPUT_R1[:], GENESIS_BLOCK_OUTPUT_R2[:]}
	GENESIS_BLOCK_SEED              = [32]byte{1}
	GENESIS_BLOCK_PHI               = [32]byte{1}
	GENESIS_BLOCK_JSPUBKEY          = "8a8ae7ff31597a4d87be0780a5c887c990c2965f454740dfc5b4177e900104c2"
	GENESIS_BLOCK_EPHEMERAL_PRIVKEY = [32]byte{1}
	GENESIS_BLOCK_EPHEMERAL_PUBKEY  = "2fe57da347cd62431528daac5fbb290730fff684afc4cfc2ed90995f58cb3b74"
	GENESIS_BLOCK_ENCRYPTED_DATA    = []string{"7833425932477757633737494346776c686c454436726864534c61785665754f363858356e6e365263354a5f5449387177455f6f72594342356f697837704a655f6c5f42346573574b735a44496b544b45714c704c7a66325274724a4657354b757742794b5f6a2d685666304c7677315f6a4e6c6346677a7448746d4a5f67453750554a555a45676e52472d754c4a6c5f675746743734566d4759356e4b4a51393473596a4b6a6176794d4e4e44592d77346f6f5a6e4d78416d5a726430576b646c5a69544b786e44637238466678626c6a5a4a3277", "3065564c4e667772573246356e767a56796459787836484572366b4d686d444c71754b66635a69495639466642476c50696a635137464a664258786b617579335a484c646a797245357550764f46687732364e7841767142676d7150763154695072703462445f584a3841785a4c777849346138627675373734526865716153634d5135376a506c556153726769776c354f75397a616e46665461546936576d64535863395f685055413143714750474774726f644a75656579714365577778"}
	GENESIS_BLOCK_G_A               = "001d24200a406ce63e3546ec62481c69dc54b1b497f1b71cb4f2d1c1f70aca3094"
	GENESIS_BLOCK_G_APrime          = "01038637efe562de043a7ffbf7db979bd4ee136d058ccef0e5962013d266c9392d"
	GENESIS_BLOCK_G_B               = "0106ad6084a7e02e23867a9f60e6f577f404739e947d7479e989f023f8151b70add3a0b82ca187c373301d0c102bb777fffc64dd8f5d73d0b0df20dd27a235f0ec"
	GENESIS_BLOCK_G_BPrime          = "0100a977a84cbeec44c11dab1733349ad6934d17b7e5c1534a138a8e97ba388a41"
	GENESIS_BLOCK_G_C               = "0116dc7fc65af2ab3808ea6a6f50a921944db737c0055471764efd9539733ae1a7"
	GENESIS_BLOCK_G_CPrime          = "00050ebd0eb393a4016bdd9f57ce77a232d8967cbfc241b644154ff2349b5ba3ac"
	GENESIS_BLOCK_G_K               = "002a79b44bd0e1302c46882807f80e5be6c1d335fd0ef06fc2292da97afba83599"
	GENESIS_BLOCK_G_H               = "0124f8d2f67d0085311a61f13831914f2e59aa4870e696cdc49ff88f349efc122d"
	GENESIS_BLOCK_VMACS             = []string{"5264e44bd87cc4d555d57069f53990e9237c10d91f32e2b0c3e5ea54a9d4c7cb", "9f1187c5cf2e999904e43595ebcce0cfde7f022747b823e7134fd389c1e5a5ad"}
)
