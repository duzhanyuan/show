package serv

import (
	"log"
	"net/http"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/inszva/show/p2p"
)

var (
	upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
)

/* Message Type:

1.Connection message from client to server
Tell the server the client is online
{
	"msg": "connect"
}
2.Start pairing from client to server
Tell the server the client want to pair a partner
{
	"msg": "pair"
}
3.Stop pairing from client to server
Tell the server the client want to stop pairing
{
	"msg": "stop_pair"
}

1.Ok message from server to client
Tell the client the server get it and ok
{
	"msg": "ok"
}
2.Update message from server to client
Tell the client to update the online num
{
	"msg": "update",
	"online_num": 7,
	"pairing_num": 5
}
3.Paired message from server to client
Tell the client to get ready to chat and its role
{
	"msg": "paired",
	"role": "offer"	//offer, answer, fail
}
4.Error message from server to clinet
Tell the client a error occur
{
	"msg": "error",
	"error": "Your state is unnormal"
}

1.Offer message from client to server to client
{
	"msg": "offer",
	"sdp": sdp
}
2.Answer message from client to server to client
{
	"msg": "answer",
	"sdp": sdp
}
*/

func HandleWebsocket(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	p := p2p.NewPoint(c, func(p, pair *p2p.Point) {
		reply := make(map[string]interface{})
		reply["msg"] = "paired"
		switch p.Status {
		case p2p.P2P_POINT_ANSWER:
			reply["role"] = "answer"
		case p2p.P2P_POINT_OFFER:
			reply["role"] = "offer"
		default:
			reply["msg"] = "error"
			reply["error"] = "Your status is unnormal"
		}
		p.Push(reply)
	})
	var data map[string]interface{}
	for p.Status != p2p.P2P_POINT_CLOSE {
		data, err = p.Pull()
		if err != nil {
			log.Println(p.Conn.RemoteAddr().String(), "leave")
			return
		}
		log.Println(data)
		msg, ok := data["msg"].(string)
		if !ok {
			continue
		}
		reply := make(map[string]interface{})
		switch msg {
		case "connect":
			reply["msg"] = "ok"
			p.Push(reply)
		case "pair":
			if p.Status != p2p.P2P_POINT_READY {
				reply["msg"] = "error"
				reply["error"] = "Your status is unnormal"
				p.Push(reply)
				break
			}
			p.RandomPair()

			reply["msg"] = "ok"
			p.Push(reply)
		case "stop_pair":
			if p.Status == p2p.P2P_POINT_PAIRING {
				atomic.AddInt32(&p2p.Pairing_num, -1)
				p.Status = p2p.P2P_POINT_READY
				if p.Pair != nil { // Unexpectly paired
					p.Pair.Status = p2p.P2P_POINT_READY
					p.Pair.Pair = nil
					p.Pair = nil
				}
			}
			reply["msg"] = "ok"
			p.Push(reply)
		case "offer":
			if p.Status != p2p.P2P_POINT_OFFER || p.Pair == nil {
				reply["msg"] = "error"
				reply["error"] = "Your status is unnormal"
			} else {
				p.Pair.Push(data)
			}
		case "answer":
			if p.Status != p2p.P2P_POINT_ANSWER || p.Pair == nil {
				reply["msg"] = "error"
				reply["error"] = "Your status is unnormal"
			} else {
				p.Pair.Push(data)
			}
		case "candidate":
			if p.Status != p2p.P2P_POINT_ANSWER && p.Status != p2p.P2P_POINT_OFFER || p.Pair == nil {
				reply["msg"] = "error"
				reply["error"] = "Your status is unnormal"
			} else {
				p.Pair.Push(data)
			}
		}
		BroadCast()
	}

}

func BroadCast() {
	reply := make(map[string]interface{})
	reply["msg"] = "update"
	reply["online_num"] = p2p.Points.Len()
	reply["pairing_num"] = atomic.LoadInt32(&p2p.Pairing_num)
	select {
	case p2p.BroadCastMessage <- reply:
	default:
	}
}
