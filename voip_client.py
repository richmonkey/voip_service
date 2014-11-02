import struct
import socket
import threading
import time
import requests
import json
import sys
import select

MSG_HEARTBEAT = 1
MSG_AUTH = 2
MSG_AUTH_STATUS = 3
MSG_IM = 4
MSG_ACK = 5
MSG_RST = 6
MSG_GROUP_NOTIFICATION = 7
MSG_GROUP_IM = 8
MSG_PEER_ACK = 9
MSG_INPUTING = 10
MSG_SUBSCRIBE_ONLINE_STATE = 11
MSG_ONLINE_STATE = 12

MSG_VOIP_CONTROL = 64
MSG_VOIP_DATA = 65


VOIP_COMMAND_DIAL = 1
VOIP_COMMAND_ACCEPT = 2
VOIP_COMMAND_CONNECTED = 3
VOIP_COMMAND_REFUSE = 4
VOIP_COMMAND_HANG_UP = 5
VOIP_COMMAND_RESET = 6
VOIP_COMMAND_TALKING = 7

HOST = "127.0.0.1"
HOST = "106.186.122.158"
class Authentication:
    def __init__(self):
        self.uid = 0

class VOIPControl:
    def __init__(self):
        self.sender = 0
        self.receiver = 0
        self.cmd = 0
        self.dial_count = 1

class VOIPData:
    def __init__(self):
        self.sender = 0
        self.receiver = 0
        self.content = ""

def send_message(cmd, seq, msg, sock):
    if cmd == MSG_AUTH:
        h = struct.pack("!iibbbb", 8, seq, cmd, 0, 0, 0)
        b = struct.pack("!q", msg.uid)
        sock.sendall(h + b)
    elif cmd == MSG_VOIP_CONTROL:
        if msg.cmd == VOIP_COMMAND_DIAL:
            length = 24
        else:
            length = 20
            
        h = struct.pack("!iibbbb", length, seq, cmd, 0, 0, 0)
        b = struct.pack("!qqi", msg.sender, msg.receiver, msg.cmd)
        t = ""
        if msg.cmd == VOIP_COMMAND_DIAL:
            t = struct.pack("!i", msg.dial_count)
        sock.sendall(h+b+t)
    elif cmd == MSG_VOIP_DATA:
        length = 16 + len(msg.content)
        h = struct.pack("!iibbbb", length, seq, cmd, 0, 0, 0)
        b = struct.pack("!qq", msg.sender, msg.receiver)
        sock.sendall(h+b+msg.content)
    else:
        print "eeeeee"

def recv_message(sock):
    buf = sock.recv(12)
    if len(buf) != 12:
        return 0, 0, None
    length, seq, cmd = struct.unpack("!iib", buf[:9])
    content = sock.recv(length)
    if len(content) != length:
        return 0, 0, None

    if cmd == MSG_AUTH_STATUS:
        status, = struct.unpack("!i", content)
        return cmd, seq, status
    elif cmd == MSG_VOIP_CONTROL:
        ctl = VOIPControl()
        ctl.sender, ctl.receiver, ctl.cmd = struct.unpack("!qqi", content[:20])
        if ctl.cmd == VOIP_COMMAND_DIAL:
            ctl.dial_count = struct.unpack("!i", content[20:24])
        return cmd, seq, ctl
    elif cmd == MSG_VOIP_DATA:
        d = VOIPData()
        d.sender, d.receiver = struct.unpack("!qq", content[:16])
        d.content = content[16:]
        return cmd, seq, d
    else:
        return cmd, seq, content


def connect_server(uid, port):
    seq = 0
    address = (HOST, port)
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)  
    sock.connect(address)
    auth = Authentication()
    auth.uid = uid
    seq = seq + 1
    send_message(MSG_AUTH, seq, auth, sock)
    cmd, _, msg = recv_message(sock)
    if cmd != MSG_AUTH_STATUS or msg != 0:
        return None, 0
    return sock, seq

def send_control(sock, seq, sender, receiver, cmd):
    ctl = VOIPControl()
    ctl.sender = sender
    ctl.receiver = receiver
    ctl.cmd = cmd
    send_message(MSG_VOIP_CONTROL, seq, ctl, sock)
    
def send_dial(sock, seq, sender, receiver):
    send_control(sock, seq, sender, receiver, VOIP_COMMAND_DIAL)
    
def send_accept(sock, seq, sender, receiver):
    send_control(sock, seq, sender, receiver, VOIP_COMMAND_ACCEPT)
    
def send_refuse(sock, seq, sender, receiver):
    send_control(sock, seq, sender, receiver, VOIP_COMMAND_REFUSE)
    
def send_connected(sock, seq, sender, receiver):
    send_control(sock, seq, sender, receiver, VOIP_COMMAND_CONNECTED)
    
def simultaneous_dial():
    caller = 86013800000009
    called = 86013800000000

    sock, seq = connect_server(caller, 20000)
    seq = seq + 1
    send_dial(sock, seq, caller, called)

    cmd, _, msg = recv_message(sock)
    if cmd != MSG_VOIP_CONTROL:
        return
    if msg.cmd == VOIP_COMMAND_ACCEPT:
        seq = seq + 1
        send_connected(sock, seq, caller, called)
        print "voip connected"
    elif msg.cmd == VOIP_COMMAND_DIAL:
        seq = seq + 1
        send_accept(sock, seq, caller, called)
        cmd, _, msg = recv_message(sock)
        if cmd != MSG_VOIP_CONTROL:
            return

        if msg.cmd == VOIP_COMMAND_CONNECTED:
            print "voip connected"
        elif msg.cmd == VOIP_COMMAND_ACCEPT:
            print "voip connected"
        else:
            return
    elif msg.cmd == VOIP_COMMAND_REFUSE:
        print "dial refused"
        return
    else:
        print "unknow:", msg.content
        return

    while True:
        cmd, _, msg = recv_message(sock)
        if cmd == MSG_VOIP_DATA:
            print "recv voip data"
            continue
        elif cmd == MSG_VOIP_CONTROL:
            print "recv voip control:", msg.cmd
            if msg.cmd == VOIP_COMMAND_HANG_UP:
                print "peer hang up"
                break
        else:
            print "unknow command:", cmd
        
def listen():
    caller = 0
    called = 86013800000009
    sock, seq = connect_server(called, 20000)
    while True:
        cmd, _, msg = recv_message(sock)
        if cmd != MSG_VOIP_CONTROL:
            continue
        if msg.cmd != VOIP_COMMAND_DIAL:
            continue
        caller = msg.sender
        break

    is_accept = query_yes_no("accept incoming dial")
    if is_accept:
        seq = seq + 1
        send_accept(sock, seq, called, caller)
        while True:
            cmd, _, msg = recv_message(sock)
            if cmd != MSG_VOIP_CONTROL:
                continue

            if msg.cmd != VOIP_COMMAND_CONNECTED:
                continue
            else:
                print "voip control:", msg.cmd
            print "voip connected caller:%d called:%d"%(caller, called)
            break
    else:
        seq = seq + 1
        send_refuse(sock, seq, called, caller)
        return

    address = ('0.0.0.0', 20001)
    s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)  
    b = struct.pack("!qq", called, caller)
    b += "\x00\x00"
    s.sendto(b, (HOST, 20001))

    while True:
        rs, _, _ = select.select([s, sock], [], [])
        if s in rs:
            data, addr = s.recvfrom(64*1024)
            sender, receiver = struct.unpack("!qq", data[:16])
            print "sender:", sender, "receiver:", receiver, " size:", len(data[16:])
        if sock in rs:
            cmd, _, msg = recv_message(sock)
            if cmd == MSG_VOIP_CONTROL:
                print "recv voip control:", msg.cmd
                if msg.cmd == VOIP_COMMAND_HANG_UP:
                    print "peer hang up"
                    break
            elif cmd == 0:
                print "voip control socket closed"
                break
            else:
                print "unknow command:", cmd

def query_yes_no(question, default="yes"):
    """Ask a yes/no question via raw_input() and return their answer.

    "question" is a string that is presented to the user.
    "default" is the presumed answer if the user just hits <Enter>.
        It must be "yes" (the default), "no" or None (meaning
        an answer is required of the user).

    The "answer" return value is one of "yes" or "no".
    """
    valid = {"yes": True, "y": True, "ye": True,
             "no": False, "n": False}
    if default is None:
        prompt = " [y/n] "
    elif default == "yes":
        prompt = " [Y/n] "
    elif default == "no":
        prompt = " [y/N] "
    else:
        raise ValueError("invalid default answer: '%s'" % default)

    while True:
        sys.stdout.write(question + prompt)
        choice = raw_input().lower()
        if default is not None and choice == '':
            return valid[default]
        elif choice in valid:
            return valid[choice]
        else:
            sys.stdout.write("Please respond with 'yes' or 'no' "
                             "(or 'y' or 'n').\n")

if __name__ == "__main__":
    #simultaneous_dial()
    listen()
