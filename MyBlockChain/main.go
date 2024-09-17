/*
  My Block Chain: main
*/
package main

import (
	"flag"
	"fmt"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"net/http"
	"strconv"

	"MyBlockChain/Block"
	"MyBlockChain/P2P"
)

const (
	HOST     = "127.0.0.1"
	API_PORT = 3000
	P2P_PORT = 4000

	INIT            = "/init/"
	BLOCKLIST       = "/blocks"
	BLOCK           = "/block/"
	NODELIST        = "/nodes"
	NODE            = "/node/"
	MALICIOUS_BLOCK = "/malicious_block/"

	debug_mode = false
)

var (
	p2p *P2P.P2PNetwork
	bc  *Block.BlockChain
)

// ブロック一覧取得
func listBlocks(c echo.Context) error {
	fmt.Println("listBlocks:")
	blocks := bc.ListBlock()
	return c.JSON(http.StatusOK, blocks)
}

// 特定のブロックの内容を取得
func getBlock(c echo.Context) error {
	id := c.Param("id")
	fmt.Println("getBlock: ", id)

	// ブロック検索
	// データの中身で検索
	block := bc.GetBlockByData([]byte(id))
	if block != nil {
		return c.JSON(http.StatusOK, block)
	}
	// indexで検索
	index, err := strconv.Atoi(id)
	if err == nil {
		block := bc.GetBlockByIndex(index)
		if block != nil {
			return c.JSON(http.StatusOK, block)
		}
	}
	// ハッシュで検索
	block = bc.GetBlock(id)
	if block != nil {
		return c.JSON(http.StatusOK, block)
	}

	return echo.NewHTTPError(http.StatusNotFound, "Block is not found.id="+id)
}

// ネットワークに接続しているサーバ一覧を取得
func listNodes(c echo.Context) error {
	fmt.Println("listNodes:")
	nodes := p2p.List()
	return c.JSON(http.StatusOK, nodes)
}

type Data struct {
	Data string `json:"data"`
}

// ブロックに記録するデータを渡し、ブロック作成を依頼する
func createBlock(c echo.Context) error {
	fmt.Println("createBlock:")

	if bc.IsMining() {
		// マイニングは同時実行しない
		return echo.NewHTTPError(http.StatusConflict, "Already Mining")
	}

	data := new(Data)
	err := c.Bind(data)
	if err != nil {
		fmt.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid data info.")
	}
	// データ保存処理
	bc.SaveData([]byte(data.Data))

	return c.NoContent(http.StatusOK)
}

// ネットワークにサーバを追加
func addNode(c echo.Context) error {
	fmt.Println("addNode:")

	node := new(P2P.Node)

	// サーバ情報取得(JOSN形式のリクエストから情報を取り出す)
	err := c.Bind(node)
	if err != nil {
		fmt.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid server info.")
	}

	// サーバ追加
	id, err := p2p.Add(node)
	if debug_mode {
		fmt.Println(id)
		fmt.Println(err)
	}

	return c.NoContent(http.StatusOK)
}

// ブロックチェーンの初期化
func initBlockChain(c echo.Context) error {
	id := c.Param("id")
	fmt.Println("initBlockChain: ", id)

	index, err := strconv.Atoi(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid Hight.")
	}
	bc.SyncBlockChain(index)

	return c.NoContent(http.StatusOK)
}

type BlockModify struct {
	Hight int    `json:"hight"`
	Data  string `json:"data"`
}

// データの書き換え
func maliciousBlock(c echo.Context) error {
	fmt.Println("maliciousBlock:")

	data := new(BlockModify)

	// リクエストデータを取得(JOSN形式のリクエストから情報を取り出す)
	err := c.Bind(data)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid data.")
	}

	bc.Modify(data.Hight, data.Data)

	return c.NoContent(http.StatusOK)
}

// バージョン番号を返す
func requestHandler(c echo.Context) error {
	return c.String(http.StatusOK, "My Block Chain Ver0.1")
}

// メイン処理
func main() {

	// オプションの解析
	apiport := flag.Int("apiport", API_PORT, "API port number")
	p2pport := flag.Int("p2pport", P2P_PORT, "P2P port number")
	host := flag.String("host", HOST, "p2p port number")
	first := flag.Bool("first", false, "first server")
	flag.Parse()

	api_port := uint16(*apiport)
	p2p_port := uint16(*p2pport)
	my_host := *host

	fmt.Println("HOST:", my_host)
	fmt.Println("API port:", api_port)
	fmt.Println("P2P port:", p2p_port)

	// P2Pモジュールの初期化
	p2p = new(P2P.P2PNetwork)
	_, err := p2p.Init(my_host, api_port, p2p_port)
	if err == nil {
		fmt.Println("P2P module initialized.")
	} else {
		fmt.Println(err)
		return
	}

	// Block Chainモジュールの初期化
	bc = new(Block.BlockChain)
	_, err = bc.Init(p2p, true) // 本当は１つ目のサーバのみtrueで他のものはfalse、そしてgenesisブロックも転送が必要
	if err == nil {
		fmt.Println("Block Chain module initialized.")
	} else {
		fmt.Println(err)
		return
	}
	if *first {
		bc.Initialized()
	}
	if debug_mode {
		fmt.Println(p2p)
		fmt.Println(bc)
	}

	// アクション登録
	p2p.SetAction(P2P.CMD_NEWBLOCK, bc.NewBlock)
	p2p.SetAction(P2P.CMD_ADDSRV, p2p.AddSrv)
	p2p.SetAction(P2P.CMD_SENDBLOCK, bc.SendBlock)
	p2p.SetAction(P2P.CMD_MININGBLOCK, bc.MiningBlock)
	p2p.SetAction(P2P.CMD_MODIFYDATA, bc.ModifyData)

	// Echoセットアップ
	e := echo.New()

	// アクセスログの設定
	e.Use(middleware.Logger())

	// エラー発生時の対処設定
	e.Use(middleware.Recover())

	// ブラウザからjavascriptを使ってAPI呼び出しできるようにCORS対応
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{echo.GET, echo.PUT, echo.POST, echo.DELETE, echo.HEAD},
	}))

	// リクエストハンドラ登録
	e.GET("/", requestHandler)
	e.GET(BLOCKLIST, listBlocks)
	e.GET(BLOCK+":id", getBlock)
	e.POST(BLOCK, createBlock)
	e.GET(NODELIST, listNodes)
	e.POST(NODE, addNode)
	e.PUT(NODE, addNode)
	e.POST(MALICIOUS_BLOCK, maliciousBlock)

	e.POST(INIT+":id", initBlockChain)

	// サーバの起動
	e.Logger.Fatal(e.Start(my_host + ":" + strconv.FormatInt(int64(api_port), 10)))
}
