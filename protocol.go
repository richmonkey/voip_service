package main

import "io"
import "bytes"
import "encoding/binary"
import log "github.com/golang/glog"

const MSG_HEARTBEAT = 1
const MSG_AUTH = 2
const MSG_AUTH_STATUS = 3
const MSG_RST = 6
const MSG_PING = 13
const MSG_PONG = 14

const MSG_VOIP_CONTROL = 64


type VOIPControl struct {
	sender   int64
	receiver int64
	content  []byte
}

type Authentication struct {
	uid int64
}

type AuthenticationStatus struct {
	status int32
}

type Message struct {
	cmd  int
	seq  int
	body interface{}
}

func ReceiveMessage(conn io.Reader) *Message {
	buff := make([]byte, 12)
	_, err := io.ReadFull(conn, buff)
	if err != nil {
		log.Info("sock read error:", err)
		return nil
	}
	var len int32
	var seq int32
	buffer := bytes.NewBuffer(buff)
	binary.Read(buffer, binary.BigEndian, &len)
	binary.Read(buffer, binary.BigEndian, &seq)
	cmd, _ := buffer.ReadByte()
	log.Info("cmd:", cmd)
	if len < 0 || len > 64*1024 {
		log.Info("invalid len:", len)
		return nil
	}
	buff = make([]byte, len)
	_, err = io.ReadFull(conn, buff)
	if err != nil {
		log.Info("sock read error:", err)
		return nil
	}

	if cmd == MSG_AUTH {
		buffer := bytes.NewBuffer(buff)
		var uid int64
		binary.Read(buffer, binary.BigEndian, &uid)
		log.Info("uid:", uid)
		return &Message{MSG_AUTH, int(seq), &Authentication{uid}}
	} else if cmd == MSG_AUTH_STATUS {
		buffer := bytes.NewBuffer(buff)
		var status int32
		binary.Read(buffer, binary.BigEndian, &status)
		return &Message{MSG_AUTH_STATUS, int(seq), &AuthenticationStatus{status}}
	} else if cmd == MSG_HEARTBEAT || cmd == MSG_PING {
		return &Message{int(cmd), int(seq), nil}
	} else if cmd == MSG_VOIP_CONTROL {
		if len <= 16 {
			return nil
		}
		ctl := &VOIPControl{}
		buffer := bytes.NewBuffer(buff)
		binary.Read(buffer, binary.BigEndian, &ctl.sender)
		binary.Read(buffer, binary.BigEndian, &ctl.receiver)
		ctl.content = buff[16:]
		return &Message{int(cmd), int(seq), ctl}
	} else {
		log.Info("invalid cmd:", cmd)
		return nil
	}
}

func WriteHeader(len int32, seq int32, cmd byte, buffer *bytes.Buffer) {
	binary.Write(buffer, binary.BigEndian, len)
	binary.Write(buffer, binary.BigEndian, seq)
	buffer.WriteByte(cmd)
	buffer.WriteByte(byte(0))
	buffer.WriteByte(byte(0))
	buffer.WriteByte(byte(0))
}


func WriteAuth(conn io.Writer, seq int, auth *Authentication) {
	var length int32 = 8
	buffer := new(bytes.Buffer)
	WriteHeader(length, int32(seq), MSG_AUTH, buffer)
	binary.Write(buffer, binary.BigEndian, auth.uid)
	buf := buffer.Bytes()
	n, err := conn.Write(buf)
	if err != nil || n != len(buf) {
		log.Info("sock write error")
	}
}

func WriteAuthStatus(conn io.Writer, seq int, auth *AuthenticationStatus) {
	var length int32 = 4
	buffer := new(bytes.Buffer)
	WriteHeader(length, int32(seq), MSG_AUTH_STATUS, buffer)
	binary.Write(buffer, binary.BigEndian, auth.status)
	buf := buffer.Bytes()
	n, err := conn.Write(buf)
	if err != nil || n != len(buf) {
		log.Info("sock write error")
	}
}


func WriteRST(conn io.Writer, seq int) {
	var length int32 = 0
	buffer := new(bytes.Buffer)
	WriteHeader(length, int32(seq), MSG_RST, buffer)
	buf := buffer.Bytes()
	n, err := conn.Write(buf)
	if err != nil || n != len(buf) {
		log.Info("sock write error")
	}
}

func WriteHeartbeat(conn io.Writer, seq int) {
	var length int32 = 0
	buffer := new(bytes.Buffer)
	WriteHeader(length, int32(seq), MSG_HEARTBEAT, buffer)
	buf := buffer.Bytes()
	n, err := conn.Write(buf)
	if err != nil || n != len(buf) {
		log.Info("sock write error", err)
	}
}

func WritePong(conn io.Writer, seq int) {
	var length int32 = 0
	buffer := new(bytes.Buffer)
	WriteHeader(length, int32(seq), MSG_PONG, buffer)
	buf := buffer.Bytes()
	n, err := conn.Write(buf)
	if err != nil || n != len(buf) {
		log.Info("sock write error", err)
	}
}

func WriteVOIPControl(conn io.Writer, seq int, ctl *VOIPControl) {
	var length int32 = int32(len(ctl.content) + 16)
	buffer := new(bytes.Buffer)
	WriteHeader(length, int32(seq), MSG_VOIP_CONTROL, buffer)
	binary.Write(buffer, binary.BigEndian, ctl.sender)
	binary.Write(buffer, binary.BigEndian, ctl.receiver)
	buffer.Write([]byte(ctl.content))
	buf := buffer.Bytes()

	n, err := conn.Write(buf)
	if err != nil || n != len(buf) {
		log.Info("sock write error")
	}
}

func SendMessage(conn io.Writer, msg *Message) {
	if msg.cmd == MSG_AUTH {
		WriteAuth(conn, msg.seq, msg.body.(*Authentication))
	} else if msg.cmd == MSG_AUTH_STATUS {
		WriteAuthStatus(conn, msg.seq, msg.body.(*AuthenticationStatus))
	} else if msg.cmd == MSG_RST {
		WriteRST(conn, msg.seq)
	} else if msg.cmd == MSG_HEARTBEAT {
		WriteHeartbeat(conn, msg.seq)
	} else if msg.cmd == MSG_PONG {
		WritePong(conn, msg.seq)
	} else if msg.cmd == MSG_VOIP_CONTROL {
		WriteVOIPControl(conn, msg.seq, msg.body.(*VOIPControl))
	} else {
		log.Info("unknow cmd", msg.cmd)
	}
}
