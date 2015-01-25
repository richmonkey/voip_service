package main

import "net"
import "time"
import "fmt"
import "bytes"
import "encoding/binary"
import "encoding/json"
import log "github.com/golang/glog"

const CLIENT_TIMEOUT = (60 * 10)

type Client struct {
	tm     time.Time
	wt     chan *Message
	uid    int64
	conn   *net.TCPConn
}

func NewClient(conn *net.TCPConn) *Client {
	client := new(Client)
	client.conn = conn
	client.wt = make(chan *Message, 10)
	return client
}

func (client *Client) Read() {
	for {
		client.conn.SetDeadline(time.Now().Add(CLIENT_TIMEOUT * time.Second))
		msg := ReceiveMessage(client.conn)
		if msg == nil {
			route.RemoveClient(client)
			client.wt <- nil
			break
		}
		log.Info("msg:", msg.cmd)
		if msg.cmd == MSG_AUTH {
			client.HandleAuth(msg.body.(*Authentication))
		} else if msg.cmd == MSG_HEARTBEAT {

		} else if msg.cmd == MSG_PING {
			client.HandlePing()
		} else if msg.cmd == MSG_VOIP_CONTROL {
			client.HandleVOIPControl(msg.body.(*VOIPControl))
		} else {
			log.Info("unknown msg:", msg.cmd)
		}
	}
}

func (client *Client) ResetClient(uid int64) {
	//单点登录
	c := route.FindClient(client.uid)
	if c != nil {
		c.wt <- &Message{cmd: MSG_RST}
	}
}

func (client *Client) SendMessage(uid int64, msg *Message) bool {
	other := route.FindClient(uid)
	if other != nil {
		other.wt <- msg
		return true
	}
	return false
}


func (client *Client) IsOnline(uid int64) bool {
	other := route.FindClient(uid)
	if other != nil {
		return true
	}
	return false
}

func (client *Client) SetUpTimestamp() {
	conn := redis_pool.Get()
	defer conn.Close()

	key := fmt.Sprintf("users_%d", client.uid)
	_, err := conn.Do("HSET", key, "up_timestamp", client.tm.Unix())
	if err != nil {
		log.Info("hset err:", err)
		return
	}
}

func (client *Client) HandleAuth(login *Authentication) {
	client.tm = time.Now()
	client.uid = login.uid
	log.Info("auth:", login.uid)
	msg := &Message{cmd: MSG_AUTH_STATUS, body: &AuthenticationStatus{0}}
	client.wt <- msg

	client.ResetClient(client.uid)

	route.AddClient(client)
	client.SetUpTimestamp()
}

func (client *Client) HandlePing() {
	msg := &Message{cmd: MSG_PONG}
	client.wt <- msg
}


const VOIP_COMMAND_DIAL = 1

func (client *Client) GetDialCount(ctl *VOIPControl) int {
	if len(ctl.content) < 4 {
		return 0
	}

	var ctl_cmd int32
	buffer := bytes.NewBuffer(ctl.content)
	binary.Read(buffer, binary.BigEndian, &ctl_cmd)
	if ctl_cmd != VOIP_COMMAND_DIAL {
		return 0
	}

	if len(ctl.content) != 8 {
		return 0
	}
	var dial_count int32
	binary.Read(buffer, binary.BigEndian, &dial_count)

	return int(dial_count)
}

func (client *Client) PublishMessage(ctl *VOIPControl) {
	//首次拨号时发送apns通知
	count := client.GetDialCount(ctl)
	if count != 1 {
		return
	}

	log.Info("publish invite notification")
	conn := redis_pool.Get()
	defer conn.Close()

	v := make(map[string]interface{})
	v["content"] = "您的朋友请求与您通话"
	v["sender"] = ctl.sender
	v["receiver"] = ctl.receiver
	b, _ := json.Marshal(v)
	_, err := conn.Do("RPUSH", "face_push_queue", b)
	if err != nil {
		log.Info("error:", err)
	}
}

func (client *Client) HandleVOIPControl(msg *VOIPControl) {
	m := &Message{cmd: MSG_VOIP_CONTROL, body: msg}
	r := client.SendMessage(msg.receiver, m)
	if !r {
		client.PublishMessage(msg)
	}
}


func (client *Client) Write() {
	seq := 0
	rst := false
	for {
		msg := <-client.wt
		if msg == nil {
			client.conn.Close()
			log.Info("socket closed")
			break
		}
		seq++
		msg.seq = seq
		if rst {
			continue
		}
		SendMessage(client.conn, msg)
		if msg.cmd == MSG_RST {
			client.conn.Close()
			rst = true
		}
	}
}

func (client *Client) Run() {
	go client.Write()
	go client.Read()
}
