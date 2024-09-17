/*
  My Block Chain: Block & Block Chain Management module
*/
package Block

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"../P2P"
)

const (
	ORPHAN_DELTA  = 300
	MAX_POW_COUNT = 60
	DIFFICULTY    = "00"
	debug_mode    = false
)

// ブロックの定義
type Block struct {
	Hight     int      `json:"hight"`
	Prev      string   `json:"prev"`
	Hash      string   `json:"hash"`
	Nonce     string   `json:"nonce"`
	PowCount  int      `json:"powcount"`
	Data      string   `json:"data"`
	Timestamp int64    `json:"timestamp"`
	Child     []*Block // このブロックの子ブロックが入る。分岐が解消されるまではここに以降のブロックが入る
	Sibling   []*Block // 同じ親を持つブロック。兄弟ブロックで、この中の１つのみが最終的に残る
}

// ブロックチェーン管理構造体
type BlockChain struct {
	Info           string
	p2p            *P2P.P2PNetwork
	initialized    bool
	mining         bool
	blocks         []*Block
	last_block     int
	fix_block      int
	orphan_blocks  []*Block
	invalid_blocks []*Block
	retry_blocks   []*Block
	mu             sync.Mutex
}


// Hash計算
func (b *Block) calcHash() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d%d%s%s%s", b.Hight, b.Prev, b.Nonce, b.PowCount, b.Data, b.Timestamp))))
}

// Hash計算してブロックに設定
func (b *Block) hash() string {
	b.Hash = b.calcHash()
	return b.Hash
}

// ブロック検証
func (b *Block) isValid() bool {

	if debug_mode {
		fmt.Println("Hash = ", b.Hash)
		fmt.Println("cal Hash = ", b.calcHash())
	}

	if b.Hash != b.calcHash() {
		return false
	}
	return true
}

// マイニング中か判断
func (bc *BlockChain) IsMining() bool {
	return bc.mining
}

// ブロックチェーン管理構造の初期化
func (bc *BlockChain) Init(p2p *P2P.P2PNetwork, first bool) (*BlockChain, error) {
	fmt.Println("Block_init")
	bc.blocks = make([]*Block, 0)
	bc.orphan_blocks = make([]*Block, 0)
	bc.invalid_blocks = make([]*Block, 0)
	bc.retry_blocks = make([]*Block, 0)
	bc.p2p = p2p
	bc.initialized = false
	bc.mining = false
	bc.Info = "My Block Chain Ver0.1"

	if first {
		// genesisブロック
		genesis_block := new(Block)
		genesis_block.Timestamp = 0
		genesis_block.Hight = 0
		genesis_block.Data = "Genesis Block"
		genesis_block.hash()
		bc.blocks = append(bc.blocks, genesis_block)
	}

	return bc, nil
}

// ブロックチェーンの同期
func (bc *BlockChain) SyncBlockChain(hight int) error {
	/* 隙間のブロックを要求 */
	bc.RequestBlock(hight)
	time.Sleep(1 * time.Second)
	bc.initialized = true
	return nil
}

// 初期化完了し動作可能とする
func (bc *BlockChain) Initialized() error {
	bc.initialized = true
	return nil
}

// 初期化完了し動作可能な状態か確認
func (bc *BlockChain) IsInitialized() bool {
	return bc.initialized
}

// ブロック作成(マイニング)
func (bc *BlockChain) Create(data string, pow bool, primary bool) (*Block, error) {

	if debug_mode {
		fmt.Println("Create:", data)
	}

	block := new(Block)
	block.Child = make([]*Block, 0)
	block.Sibling = make([]*Block, 0)

	// 競合、フォークを解消するために、一番長いチェーンの後につなげるようにする
	last_block := bc.getPrevBlock()

	// ブロックの中身を詰める
	block.Prev = last_block.Hash
	block.Timestamp = time.Now().UnixNano()
	block.Data = data
	block.Hight = last_block.Hight + 1

	// PoW
	if pow {
		/*
		Nonceを変えながら、条件を満たすハッシュを計算するループを回す。
		実験では、あまり終わらないと大変なので、60回(60秒)やってだめなら、とりえず進むことにする。
		*/
		for i := 0; i < MAX_POW_COUNT; i++ {
			block.Nonce = fmt.Sprintf("%x", rand.New(rand.NewSource(block.Timestamp/int64(i+1))))
			block.PowCount = i
			block.hash()
			if debug_mode {
				fmt.Println("Try ", i, block)
			} else {
				fmt.Println("Try ", i, block.Hash)
			}
			// 求めたハッシュが条件を満たすか確認する
			if strings.HasPrefix(block.Hash, DIFFICULTY) {
				fmt.Println("Found!!")
				break
			}
			time.Sleep(1 * time.Second)
		}
		if primary == false && !strings.HasPrefix(block.Hash, DIFFICULTY) {
			return nil, errors.New("Failed to Mine.")
		}
	} else {
		block.hash()
	}

	return block, nil
}

// チェーンの親ブロックを見つける
func (bc *BlockChain) getPrevBlock() *Block {

	// 一番長いチェーンから親を決める
	// ロック
	bc.mu.Lock()

	last_block := bc.blocks[len(bc.blocks)-1]

	block := last_block
	if len(last_block.Sibling) > 0 {
		for _, b := range last_block.Sibling {
			if len(block.Child) < len(b.Child) {
				block = b
			}
		}
	}

	// アンロック
	bc.mu.Unlock()

	return block
}

// ブロックチェーンの整合性確認
func (bc *BlockChain) Check(data []byte) error {
	fmt.Println("Checking My Block Chain...")
	bc.mu.Lock()
	prev := bc.blocks[0]
	for i := 1; i < len(bc.blocks); i++ {
		fmt.Println(".")
		if prev.calcHash() != bc.blocks[i].Prev {
			fmt.Println("Invalid Block Found!")
			fmt.Println(prev)
			bc.invalid_blocks = append(bc.invalid_blocks, prev)
		}
		prev = bc.blocks[i]
	}
	bc.mu.Unlock()
	fmt.Println("... Done")

	return nil
}


// ブロックをチェーンにつなぐ
func (bc *BlockChain) blockAppendSimple(block *Block) error {
	if debug_mode {
		fmt.Println("blockAppendSimple:", block)
	}
	// チェーンの最後
	last_block := bc.blocks[len(bc.blocks)-1]
	// Blockの親がblocksの最後か？
	if block.Prev == last_block.Hash {
		// つなぐ
		bc.blocks = append(bc.blocks, block)
	} else if last_block.Prev == block.Prev {
		if last_block.Timestamp > block.Timestamp {
			// 入れ替え＆last_block解放
			bc.blocks[len(bc.blocks)-1] = block
			fmt.Println("Purge Block:", last_block)
		}
	} else if block.Hight > last_block.Hight {
		// 親がいなければorphanにつなぐ
		bc.orphan_blocks = append(bc.orphan_blocks, block)

		// 隙間があったら、間のブロックの送信を依頼
		for i := last_block.Hight + 1; i < block.Hight; i++ {
			/* 隙間のブロックを要求 */
			bc.RequestBlock(i)
			time.Sleep(1 * time.Second / 2)
		}
	} else {
		// それ以外がチェーンに繋げないので破棄
		fmt.Println("Purge Block:", block)
	}

	return nil

}

// ブロックをつなぐ
func (bc *BlockChain) AddBlock(block *Block) error {

	if debug_mode {
		fmt.Println("AddBlock:", block)
	}

	// ロック
	bc.mu.Lock()

	// ブロックをチェーンにつなぐ
	err := bc.blockAppendSimple(block)
	if err != nil {
		// アンロック
		bc.mu.Unlock()
		return err
	}

	// orphan_blocksに繋がっているものの親が繋がったか確認する
	last_block := bc.blocks[len(bc.blocks)-1]
	for i, b := range bc.orphan_blocks {
		if b.Prev == last_block.Hash {
			if debug_mode {
				fmt.Println("retry")
				fmt.Println("list block before")
				bc.DumpChain()
			}

			// orphan_blocksから外す
			bc.orphan_blocks = append(bc.orphan_blocks[:i], bc.orphan_blocks[i+1:]...)

			// ブロックをチェーンにつなぐ
			bc.blockAppendSimple(b)
			if debug_mode {
				fmt.Println(b)
				fmt.Println("list block after")
				bc.DumpChain()
			}
		}
	}

	// アンロック
	bc.mu.Unlock()

	return nil
}

// ブロックを要求
func (bc *BlockChain) RequestBlock(id int) error {
	fmt.Println("RequestBlock:", id)
	bid := make([]byte, 4)
	binary.LittleEndian.PutUint32(bid, uint32(id))
	node := []byte(bc.p2p.Self())
	s_msg := append(bid, node...)
	fmt.Println(s_msg)
	bc.p2p.SendOne(P2P.CMD_SENDBLOCK, s_msg)
	return nil
}

// ハッシュ指定でブロックを取得
func (bc *BlockChain) GetBlock(hash string) *Block {
	fmt.Println("GetBlock:", hash)
	bc.mu.Lock()
	for _, b := range bc.blocks {
		fmt.Println(b)
		if b.Hash == hash {
			bc.mu.Unlock()
			fmt.Println("GetBlock: Found", b)
			return b
		}
	}
	bc.mu.Unlock()
	return nil
}

// インデックス指定でブロックを取得
func (bc *BlockChain) GetBlockByIndex(index int) *Block {
	fmt.Println("GetBlockByIndex:", index)
	bc.mu.Lock()
	if len(bc.blocks) > index {
		b := bc.blocks[index]
		bc.mu.Unlock()
		fmt.Println("GetBlockByIndex: Found", b)
		return b
	}
	bc.mu.Unlock()
	return nil
}

// データ指定でブロックを取得
func (bc *BlockChain) GetBlockByData(data []byte) *Block {
	bc.mu.Lock()
	for _, b := range bc.blocks {
		if b.Data == string(data) {
			bc.mu.Unlock()
			fmt.Println("GetBlockByData: Found", b)
			return b
		}
	}
	bc.mu.Unlock()
	return nil
}

// ブロック一覧を取得
func (bc *BlockChain) ListBlock() []*Block {
	fmt.Println("ListBlock:")
	fmt.Println("  blocks->")
	bc.mu.Lock()
	for _, b := range bc.blocks {
		fmt.Println("    ", b)
	}
	fmt.Println("  orphan_blocks->")
	for _, b := range bc.orphan_blocks {
		fmt.Println("    ", b)
	}
	fmt.Println("----------")
	bc.mu.Unlock()
	return bc.blocks
}

/***** デバッグ用 *******/
func (bc *BlockChain) DumpChain() {
	fmt.Println("----------------")
	fmt.Println("Info => ", bc.Info)

	fmt.Println("ListBlock:")
	fmt.Println("  blocks->")
	for _, b := range bc.blocks {
		//fmt.Println("    ", b)
		fmt.Println("    ", b.Data)
	}
	fmt.Println("  orphan_blocks->")
	for _, b := range bc.orphan_blocks {
		//fmt.Println("    ", b)
		fmt.Println("    ", b.Data)
	}
	fmt.Println("----------------")
	return
}

/************************/

// 新しいブロックの承認＆追加アクション
func (bc *BlockChain) NewBlock(msg []byte) error {
	fmt.Println("new block action")
	//     fmt.Println(msg)
	//     fmt.Println(string(msg))

	// ブロックを取り出す
	block := new(Block)
	err := json.Unmarshal(msg, block)
	if err != nil {
		fmt.Println("Invalid Block.", err)
		return errors.New("Invalid Block.")
	}
	if debug_mode {fmt.Println("block = ", block)}

	// Check
	if block.isValid() == false {
		/* 不正なブロックなのでつながない */
		return errors.New("Invalid Block: ID=" + strconv.FormatInt(int64(block.Hight), 10))
	}

	// チェーンにつなぐ
	bc.AddBlock(block)

	return nil
}

// ブロック送信のアクション
func (bc *BlockChain) SendBlock(msg []byte) error {
	fmt.Println("send block action")

	if debug_mode {
		fmt.Println(msg)
	}

	// メッセージ解析
	var block_id uint32
	buf := bytes.NewReader(msg)
	err := binary.Read(buf, binary.LittleEndian, &block_id)
	fmt.Println(err)
	target := string(msg[4:])
	fmt.Println(block_id, target)

	// ブロック取得
	block := bc.GetBlockByIndex(int(block_id))
	if block == nil {
		fmt.Println("Invalid Block ID")
		return errors.New("Invalid Block ID:" + strconv.FormatInt(int64(block_id), 10))
	}

	// 送信先を特定
	srv := strings.Split(target, ":")
	port, _ := strconv.Atoi(srv[1])
	node := bc.p2p.Search(srv[0], uint16(port))
	if node == nil {
		fmt.Println("Node NOT Found:" + target)
		return errors.New("Node NOT Found:" + target)
	}

	// ブロック送信
	b, _ := json.Marshal(block)

	if debug_mode {
		fmt.Println("block = ", block)
		fmt.Println("b = ", b)
	}

	// 新しいブロックを要求元サーバに送る
	s_msg := append([]byte{byte(P2P.CMD_NEWBLOCK)}, b...)
	node.Send(s_msg)

	return nil
}

// マイニング処理
func (bc *BlockChain) miningBlock(data []byte, primary bool) error {
	if debug_mode {
		fmt.Println("MiningBlock:", data)
	}

	bc.mu.Lock()
	if bc.initialized == false {
		bc.mu.Unlock()
		fmt.Println("Could not start mining.")
		return errors.New("Could not start mining.")
	}
	if bc.mining {
		bc.mu.Unlock()
		fmt.Println("Someone Mining.")
		return errors.New("Someone Mining.")
	}
	bc.mining = true
	bc.mu.Unlock()

	// ブロックに記録するデータ取り出し
	d := data

	// マイニング
	block, err := bc.Create(string(d), true, primary)
	if err == nil {
		b, _ := json.Marshal(block)
		if debug_mode {
			fmt.Println(b)
		}
		// 全ノードに保存要求を送る
		bc.p2p.Broadcast(P2P.CMD_NEWBLOCK, b, true)
	}

	bc.mu.Lock()
	bc.mining = false
	bc.mu.Unlock()
	return err
}

// マイニングアクション
func (bc *BlockChain) MiningBlock(data []byte) error {
	return bc.miningBlock(data, false)
}

// データ保存リクエスト
func (bc *BlockChain) SaveData(data []byte) error {

	fmt.Println("SaveData:", data)

	// 全ノードにマイニング要求を送る
	bc.p2p.Broadcast(P2P.CMD_MININGBLOCK, data, false)

	// 自身のマイニング
	go bc.miningBlock(data, true)

	return nil
}

// データ書き換えアクション
func (bc *BlockChain) ModifyData(msg []byte) error {

	fmt.Println("ModifyData:", msg)

	// get hight
	var hight uint32
	buf := bytes.NewReader(msg)
	err := binary.Read(buf, binary.LittleEndian, &hight)
	if err != nil {
		return errors.New("Invalid request.")
	}

	// get data
	data := string(msg[4:])
	if debug_mode {
		fmt.Println(data)
	}

	block := new(Block)
	target := bc.GetBlockByIndex(int(hight))
	if target == nil {
		return errors.New("No target Block: ID=" + strconv.FormatInt(int64(hight), 10))
	}

	// ブロックの内容をコピー
	*block = *target

	// 無理やりデータを変更&チェック
	block.Data = data
	fmt.Println(block)
	if block.isValid() == false {
	   // 不正なブロックなので、書き換えをやめる
		fmt.Println("Invalid Block!:", block)
		return errors.New("Invalid Block: ID=" + strconv.FormatInt(int64(block.Hight), 10))
	} else {
		// データを書き換え
		*target = *block
		// 一応全体チェックをかける
		d := make([]byte, 4)
		bc.Check(d)
	}

	return nil
}

// データ書き換えリクエスト
func (bc *BlockChain) Modify(hight int, data string) error {

	fmt.Println("Modify:", hight, data)

	// メッセージ組み立て
	idx := make([]byte, 4)
	binary.LittleEndian.PutUint32(idx, uint32(hight))
	s_msg := append(idx, data...)
	fmt.Println("s_msg = ", s_msg)

	// 全ノードにデータ書き換え要求を送る
	bc.p2p.Broadcast(P2P.CMD_MODIFYDATA, s_msg, true)

	return nil
}
