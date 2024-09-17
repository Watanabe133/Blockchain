/*
  My Block Chain: P2P Network module
*/
package P2P

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"
)

const (
	CMD_NEWBLOCK    = 1
	CMD_ADDSRV      = 2
	CMD_DELSRV      = 3
	CMD_SENDBLOCK   = 4
	CMD_MININGBLOCK = 5
	CMD_MODIFYDATA  = 6

	debug_mode = false
)

// サーバ管理の構造体
type Node struct {
	Host    string   `json:"host" form:"host" query:"host"`
	ApiPort uint16   `json:"api_port" form :"api_port" query:"api_port"`
	P2PPort uint16   `json:"p2p_port" form :"p2p_port" query:"p2p_port"`
	Self    bool     `json:"-"`
	Conn    net.Conn `json:"-"`
}

// ネットワーク接続
func (node *Node) connect() {
	target := node.Host + ":" + strconv.Itoa(int(node.P2PPort))

	if debug_mode {
		fmt.Println("target = ", target)
	}

	conn, err := net.Dial("udp", target)
	if err != nil {
		fmt.Println("failed to connect ", target, err)
		node.Conn = nil
	} else {
		fmt.Println(target, "connected.")
		node.Conn = conn
	}
}

// ネットワーク切断
func (node *Node) disconnect() {
	if node.Conn != nil {
		node.Conn.Close()
	}
}

// メッセージ送信
func (node *Node) Send(msg []byte) error {
	fmt.Println("Send to ", node.me(), ":", string(msg), len(msg))

	err := error(nil)
	if node.Conn != nil {
		n, err := node.Conn.Write(msg)
		if err != nil {
			fmt.Println("Write error:", n, err)
		}
	} else {
		err = errors.New("Not connected:" + node.me())
		fmt.Println("Not connected:", node.me())
	}

	return err
}

// サーバのアドレス情報の組み立て
func (node *Node) me() string {
	return node.Host + ":" + strconv.FormatInt(int64(node.P2PPort), 10)
}

type act_fn func([]byte) error

// 自身のアドレス情報を返す
func (p2p *P2PNetwork) Self() string {
	for _, n := range p2p.nodes {
		if n.Self {
			return n.me()
		}
	}
	return ""
}

// P2P通信のサーバ処理
func (p2p *P2PNetwork) p2p_srv(host string, port uint16) {
	fmt.Println("Start p2p server", host, port)

	udpAddr := &net.UDPAddr{
		IP:   net.ParseIP(host),
		Port: int(port),
	}
	updLn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		fmt.Println("listen error", err)
		return
	}

	for {
		buf := make([]byte, 1024)
		if debug_mode {
			fmt.Println("call updLn.ReadFromUDP")
		}
		n, addr, err := updLn.ReadFromUDP(buf)
		if debug_mode {
			fmt.Println("read", n, err)
		}
		if err == nil {
			go func() {
				if debug_mode {
					fmt.Println("recieve function")
					fmt.Println(addr)
					fmt.Println(string(buf))
				}
				cmd := int(buf[0])
				if debug_mode {
					fmt.Println(cmd)
				}
				msg := buf[1:n]
				if cmd < len(p2p.actions) {
					fmt.Println("Do Action")
					f := p2p.actions[cmd]
					if f != nil {
						err := f(msg)
						if err != nil {
							fmt.Println(err)
						}
					}
				} else {
					fmt.Println("No Action")
				}

			}()
		}
	}
}

// P2Pネットワーク管理構造体
type P2PNetwork struct {
	nodes   []*Node
	actions []act_fn
}

// P2Pネットワークにサーバを追加
func (p2p *P2PNetwork) Add(node *Node) (int, error) {

	fmt.Println("P2PNetwork.Add")

	fmt.Println("add node:", node)

	// 他のサーバにも追加リクエストを飛ばす
	bytes, _ := json.Marshal(node)
	p2p.Broadcast(CMD_ADDSRV, bytes, false)

	// 通信準備
	node.connect()

	// 追加されたサーバに他のサーバ情報を送る
	for _, n := range p2p.nodes {
		b, _ := json.Marshal(n)
		s_msg := append([]byte{byte(CMD_ADDSRV)}, b...)
		node.Send(s_msg)
		time.Sleep(1 * time.Second / 2)
	}

	// サーバリストに追加
	p2p.nodes = append(p2p.nodes, node)

	return 0, nil
}

// サーバ情報の検索
func (p2p *P2PNetwork) Search(host string, p2p_port uint16) *Node {

	fmt.Println("Search:", host, p2p_port)

	for _, node := range p2p.nodes {
		if node.Host == host && node.P2PPort == p2p_port {
			return node
		}
	}

	return nil
}

// P2Pネットワークに接続しているサーバ一覧を取得
func (p2p *P2PNetwork) List() []*Node {
	for _, node := range p2p.nodes {
		fmt.Println(node)
	}
	return p2p.nodes
}

// P2Pネットワークに接続しているサーバにメッセージ送信
func (p2p *P2PNetwork) Broadcast(cmd int, msg []byte, self bool) {

	fmt.Println("Broadcast:", cmd, string(msg))
	/*
	   メッセージ送信時は、cmd + msg で送る。
	   cmd は 1バイトとする。
	*/
	s_msg := append([]byte{byte(cmd)}, msg...)
	if debug_mode {
		fmt.Println(s_msg)
	}

	for _, node := range p2p.nodes {
		if debug_mode {
			fmt.Println(node)
		}
		if self == false && node.Self {
			fmt.Println("not send")
			continue
		} else {
			if err := node.Send(s_msg); err != nil {
				fmt.Println("send error:", node, err)
			}
		}
		time.Sleep(1 * time.Second / 2)
	}
}

// メッセージをいずれかのサーバに送信
func (p2p *P2PNetwork) SendOne(cmd int, msg []byte) {
	fmt.Println("SendOne:", cmd, string(msg))

	s_msg := append([]byte{byte(cmd)}, msg...)
	if debug_mode {
		fmt.Println(s_msg)
	}

	for _, node := range p2p.nodes {
		if debug_mode {
			fmt.Println(node)
		}
		if node.Self {
			fmt.Println("not send")
			continue
		} else {
			err := node.Send(s_msg)
			if err == nil {
				break
			}
		}
		time.Sleep(1 * time.Second / 2)
	}
}

// アクションとアクションハンドラの紐付け登録
func (p2p *P2PNetwork) SetAction(cmd int, handler act_fn) *act_fn {

	fn := p2p.actions[cmd]
	p2p.actions[cmd] = handler
	return &fn
}

// P2P ネットワークの初期化処理
func (p2p *P2PNetwork) Init(host string, api_port uint16, p2p_port uint16) (*P2PNetwork, error) {

	fmt.Println("P2P_init")
	p2p.nodes = make([]*Node, 0)
	p2p.actions = make([]act_fn, 20)

	// 自ノードの管理構造を初期化
	node := new(Node)
	node.Host = host
	node.ApiPort = api_port
	node.P2PPort = p2p_port
	node.Self = true

	// 自ノードの通信路開設
	node.connect()

	// サーバリストに自ノードを追加
	p2p.nodes = append(p2p.nodes, node)

	// サーバ初期化
	go p2p.p2p_srv(host, p2p_port)

	if debug_mode {
		fmt.Println(p2p)
	}

	return p2p, nil
}

// サーバ追加アクション
func (p2p *P2PNetwork) AddSrv(msg []byte) error {
	fmt.Println("add server action")

	node := new(Node)

	if debug_mode {
		fmt.Println(msg)
		fmt.Println(string(msg))
	}

	err := json.Unmarshal(msg, node)
	if err != nil {
		fmt.Println("json.Unmarshal failed")
		return err
	}
	fmt.Println("node:", node)

	/*
	   サーバリストに追加して、通信路を接続する。
	   追加するとき、Selfはfalseにすること
	*/
	node.Self = false
	node.connect()
	p2p.nodes = append(p2p.nodes, node)

	return nil
}

