/**
 * Copyright (c) 2014-2015, GoBelieve     
 * All rights reserved.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program; if not, write to the Free Software
 * Foundation, Inc., 59 Temple Place, Suite 330, Boston, MA  02111-1307  USA
 */

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
	appid  int64
	device_id string
	platform_id int8
	conn   *net.TCPConn
	public_ip int32
}

func NewClient(conn *net.TCPConn) *Client {
	client := new(Client)
	client.conn = conn
	client.wt = make(chan *Message, 10)
	addr := conn.LocalAddr()
	if taddr, ok := addr.(*net.TCPAddr); ok {
		ip4 := taddr.IP.To4()
		client.public_ip = int32(ip4[0]) << 24 | int32(ip4[1]) << 16 | int32(ip4[2]) << 8 | int32(ip4[3])
	}
	return client
}

func (client *Client) RemoveClient() {
	route := app_route.FindRoute(client.appid)
	if route == nil {
		log.Warning("can't find app route")
		return
	}
	route.RemoveClient(client)
}


func (client *Client) Read() {
	for {
		client.conn.SetDeadline(time.Now().Add(CLIENT_TIMEOUT * time.Second))
		msg := ReceiveMessage(client.conn)
		if msg == nil {
			client.wt <- nil
			client.RemoveClient()
			break
		}
		log.Info("msg:", msg.cmd)
		if msg.cmd == MSG_AUTH {
			client.HandleAuth(msg.body.(*Authentication))
		} else if msg.cmd == MSG_AUTH_TOKEN {
			client.HandleAuthToken(msg.body.(*AuthenticationToken))
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

func (client *Client) SendMessage(uid int64, msg *Message) bool {
	route := app_route.FindRoute(client.appid)
	if route == nil {
		log.Warning("can't find app route, msg cmd:", Command(msg.cmd))
		return false
	}
	clients := route.FindClientSet(uid)
	if clients != nil {
		for c, _ := range(clients) {
			c.wt <- msg
		}
		return true
	}
	return false
}


func (client *Client) AddClient() {
	route := app_route.FindOrAddRoute(client.appid)
	route.AddClient(client)
}

func (client *Client) AuthToken(token string) (int64, int64, error) {
	appid, uid, _, err := LoadUserAccessToken(token)
	return appid, uid, err
}

func (client *Client) HandleAuthToken(login *AuthenticationToken) {
	appid, uid, err := client.AuthToken(login.token)
	if err != nil {
		log.Info("auth token err:", err)
		msg := &Message{cmd: MSG_AUTH_STATUS, body: &AuthenticationStatus{1, 0}}
		client.wt <- msg
		return
	}
	if uid == 0 || appid == 0 {
		log.Info("auth token appid==0, uid==0")
		msg := &Message{cmd: MSG_AUTH_STATUS, body: &AuthenticationStatus{1, 0}}
		client.wt <- msg
		return
	}

	client.tm = time.Now()
	client.uid = uid
	client.appid = appid
	log.Infof("auth appid:%d uid:%d\n", appid, uid)

	msg := &Message{cmd: MSG_AUTH_STATUS, body: &AuthenticationStatus{0, client.public_ip}}
	client.wt <- msg

	client.SendLoginPoint()
	client.AddClient()
}


func (client *Client) HandleAuth(login *Authentication) {
	client.tm = time.Now()
	client.appid = 1006
	client.uid = login.uid
	log.Info("auth:", login.uid)
	msg := &Message{cmd: MSG_AUTH_STATUS, body: &AuthenticationStatus{0, 0}}
	client.wt <- msg

	client.AddClient()
}

func (client *Client) SendLoginPoint() {
	point := &LoginPoint{}
	point.up_timestamp = int32(client.tm.Unix())
	point.platform_id = client.platform_id
	point.device_id = client.device_id
	msg := &Message{cmd:MSG_LOGIN_POINT, body:point}
	client.SendMessage(client.uid, msg)
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


func (client *Client) IsROMApp(appid int64) bool {
	return false
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
	v["sender"] = ctl.sender
	v["receiver"] = ctl.receiver
	v["appid"] = client.appid
	b, _ := json.Marshal(v)

	appid := client.appid
	var queue_name string
	if client.IsROMApp(appid) {
		queue_name = fmt.Sprintf("voip_push_queue_%d", appid)
	} else {
		queue_name = "voip_push_queue"
	}

	_, err := conn.Do("RPUSH", queue_name, b)
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
	for {
		msg := <-client.wt
		if msg == nil {
			client.conn.Close()
			log.Info("socket closed")
			break
		}
		seq++
		msg.seq = seq
	
		SendMessage(client.conn, msg)
	}
}

func (client *Client) Run() {
	go client.Write()
	go client.Read()
}
