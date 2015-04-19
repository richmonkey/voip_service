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

import "time"
import "fmt"
import "net"
import "bytes"
import "sync"
import "encoding/binary"
import log "github.com/golang/glog"

const VOIP_AUTH = 1
const VOIP_AUTH_STATUS = 2
const VOIP_DATA = 3

const GC_HZ = 5*60
const VOIP_CLIENT_TIMEOUT = 1*60

type TunnelClient struct {
	appid     int64
	uid       int64
	addr      *net.UDPAddr
	timestamp int64
	has_header  bool
}

type TunnelClientSet map[int64]*TunnelClient


type Tunnel struct {
	app_clients map[int64]TunnelClientSet
	clients map[int64]*TunnelClient
	mutex   sync.Mutex
	gc_ts   int64
}

func NewTunnel() *Tunnel {
	t := new(Tunnel)
	t.clients = make(map[int64]*TunnelClient)
	t.app_clients = make(map[int64]TunnelClientSet)
	return t
}

func (tunnel *Tunnel) Start() {
	go tunnel.Run()
	go tunnel.RunV2()
}

func (tunnel *Tunnel) ReadVOIPData(buff []byte) (int64, int64, []byte, error) {
	buffer := bytes.NewBuffer(buff)
	var sender int64
	var receiver int64
	binary.Read(buffer, binary.BigEndian, &sender)
	binary.Read(buffer, binary.BigEndian, &receiver)
	
	return sender, receiver, buff[16:], nil
}

func (tunnel *Tunnel) ReadVOIPAuth(buff []byte) (string, error) {
	buffer := bytes.NewBuffer(buff)
	var size int16
	binary.Read(buffer, binary.BigEndian, &size)
	token := buff[2:2+size]
	return string(token), nil
}

func (tunnel *Tunnel) HandleVOIPData(buff []byte, addr *net.UDPAddr, conn *net.UDPConn) {
	now := time.Now().Unix()


	_, receiver, _, err := tunnel.ReadVOIPData(buff)
	if err != nil {
		return
	}
	
	client := tunnel.FindClient(addr)
	if client == nil || client.appid == 0 {
		return
	}
	client.timestamp = now
	//转发消息
	other := tunnel.FindAppClient(client.appid, receiver)
	if other == nil {
		return
	}

	if other.has_header {
		buffer := new(bytes.Buffer)
		var h byte = VOIP_DATA
		buffer.WriteByte(h)
		buffer.Write(buff)
		data := buffer.Bytes()
		conn.WriteTo(data, other.addr)
	} else {
		data := buff
		conn.WriteTo(data, other.addr)
	}
}

func (tunnel *Tunnel) HandleAuth(buff []byte, addr *net.UDPAddr, conn *net.UDPConn) {
	now := time.Now().Unix()
	token, err := tunnel.ReadVOIPAuth(buff)
	if err != nil {
		return
	}
	client := tunnel.FindClient(addr)
	if client == nil {
		//首次收到认证消息
		client = &TunnelClient{appid:0, uid:0, addr:addr, timestamp:now, has_header:true}
		tunnel.AddClient(addr, client)
		tunnel.AuthClient(client, token)
		return
	} else if client.appid == 0 {
		//认证中
		return
	} else {
		//认证成功
		t := make([]byte, 2)
		t[0] = VOIP_AUTH_STATUS
		t[1] = 0
		conn.WriteTo(t, addr)
		log.Infof("tunnel auth appid:%d uid:%d", client.appid, client.uid)
	}	
}

func (tunnel *Tunnel) Addr2Int64(addr *net.UDPAddr) int64 {
	ip := addr.IP.To4()
	if ip == nil {
		return 0
	}
	return (int64(ip[0]) << 24) | (int64(ip[1]) << 16) | (int64(ip[2]) << 8) | int64(ip[3]) | (int64(addr.Port) << 32)
}

func (tunnel *Tunnel) AddClient(addr *net.UDPAddr, c *TunnelClient) {
	tunnel.mutex.Lock()
	defer tunnel.mutex.Unlock()

	iaddr := tunnel.Addr2Int64(addr)
	tunnel.clients[iaddr] = c
}

func (tunnel *Tunnel) FindClient(addr *net.UDPAddr) *TunnelClient {
	iaddr := tunnel.Addr2Int64(addr)
	tunnel.mutex.Lock()
	defer tunnel.mutex.Unlock()

	return tunnel.clients[iaddr]
}

func (tunnel *Tunnel) AddAppClient(c *TunnelClient) {
	tunnel.mutex.Lock()
	defer tunnel.mutex.Unlock()

	if client_set, ok := tunnel.app_clients[c.appid]; ok {
		client_set[c.uid] = c
		return
	}
	client_set := make(map[int64]*TunnelClient)
	client_set[c.uid] = c
	tunnel.app_clients[c.appid] = client_set
}

func (tunnel *Tunnel) FindAppClient(appid int64, uid int64) *TunnelClient {
	tunnel.mutex.Lock()
	defer tunnel.mutex.Unlock()
	
	if client_set, ok := tunnel.app_clients[appid]; ok {
		if client, ok := client_set[uid]; ok {
			return client
		}
	}
	return nil
}

func (tunnel *Tunnel) AuthClient(client *TunnelClient, token string) {
	go func() {
		appid, uid, _, err := LoadUserAccessToken(token)
		if err != nil {
			log.Warning("auth token err:", err)
			return
		}

		client.appid = appid
		client.uid = uid
		tunnel.AddAppClient(client)
	}()
}




func (tunnel *Tunnel) GC() {
	now := time.Now().Unix()
	if now - tunnel.gc_ts < GC_HZ {
		return
	}

	tunnel.mutex.Lock()
	defer tunnel.mutex.Unlock()

	for k, c := range tunnel.clients {
		if now-c.timestamp > VOIP_CLIENT_TIMEOUT {
			delete(tunnel.clients, k)
			if s, ok := tunnel.app_clients[c.appid]; ok {
				delete(s, c.uid)
			}
		}
	}
	tunnel.gc_ts = now
}


func (tunnel *Tunnel) HandleData(buff []byte, addr *net.UDPAddr, conn *net.UDPConn) {
	h := buff[0]
	cmd := h&0x0f
	if cmd == VOIP_AUTH {
		tunnel.HandleAuth(buff[1:], addr, conn)
	} else if cmd == VOIP_DATA {
		tunnel.HandleVOIPData(buff[1:], addr, conn)
	}
}

func (tunnel *Tunnel) RunV2() {

	addr := fmt.Sprintf(":%d", config.tunnel_port_v2)
	laddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Fatal("resolve udp addr err:", err)
	}

	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		log.Fatal("listen upd err:", err)
	}

	buff := make([]byte, 64*1024)
	for {
		n, raddr, err := conn.ReadFromUDP(buff)
		if err != nil {
			log.Warning("read udp err:", err)
			continue
		}

		tunnel.HandleData(buff[:n], raddr, conn)

		tunnel.GC()
	}
}

//兼容旧版本电话虫
func (tunnel *Tunnel) Run() {
	addr := fmt.Sprintf(":%d", config.tunnel_port)
	laddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Fatal("resolve udp addr err:", err)
	}

	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		log.Fatal("listen upd err:", err)
	}

	buff := make([]byte, 64*1024)

	for {
		n, raddr, err := conn.ReadFromUDP(buff)
		if err != nil {
			log.Warning("read udp err:", err)
			continue
		}
		now := time.Now().Unix()

		sender, _, _, _ := tunnel.ReadVOIPData(buff[:n])
		appid := int64(1006)
		client := tunnel.FindClient(raddr)
		if client == nil {
			client = &TunnelClient{appid:appid, uid:sender, addr:raddr, timestamp:now, has_header:false}
			tunnel.AddClient(raddr, client)
			tunnel.AddAppClient(client)
		} else {
			client.timestamp = now
		}

		tunnel.HandleVOIPData(buff[:n], raddr, conn)
		tunnel.GC()
	}
}
