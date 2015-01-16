package main

import "time"
import "fmt"
import "net"
import "bytes"
import "encoding/binary"
import log "github.com/golang/glog"

type TunnelClient struct {
	uid       int64
	addr      *net.UDPAddr
	timestamp int64
}

type Tunnel struct {
}

func NewTunnel() *Tunnel {
	t := new(Tunnel)
	return t
}

func (tunnel *Tunnel) Start() {
	go tunnel.Run()
}

func (tunnel *Tunnel) ReadVOIPData(buff []byte) (int64, int64) {
	buffer := bytes.NewBuffer(buff)
	var sender int64
	var receiver int64
	binary.Read(buffer, binary.BigEndian, &sender)
	binary.Read(buffer, binary.BigEndian, &receiver)
	return sender, receiver
}

func (tunnel *Tunnel) Run() {
	clients := make(map[int64]*TunnelClient)

	addr := fmt.Sprintf(":%d", config.tunnel_port)
	laddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Fatal("resolve udp addr err:", err)
	}

	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		log.Fatal("listen upd err:", err)
	}

	timestamp := time.Now().Unix()
	buff := make([]byte, 64*1024)
	for {
		n, raddr, err := conn.ReadFromUDP(buff)
		if err != nil {
			log.Warning("read udp err:", err)
			continue
		}

		if n <= 16 {
			log.Warning("invalid packet len:", n)
			continue
		}

		now := time.Now().Unix()
		sender, receiver := tunnel.ReadVOIPData(buff[:16])
		if c, ok := clients[sender]; !ok {
			clients[sender] = &TunnelClient{sender, raddr, now}
		} else {
			c.timestamp = now
			c.addr = raddr
		}

		if c, ok := clients[receiver]; ok {
			conn.WriteTo(buff[:n], c.addr)
		}

		//清理不再通讯的客户端
		if now-timestamp > 5*60 {
			for uid, c := range clients {
				if now-c.timestamp > 1*60 {
					delete(clients, uid)
				}
			}
		}
	}
}
