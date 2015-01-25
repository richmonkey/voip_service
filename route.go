package main

import "sync"
import log "github.com/golang/glog"


type Route struct {
	mutex   sync.Mutex
	clients map[int64]*Client
}

func NewRoute() *Route {
	route := new(Route)
	route.clients = make(map[int64]*Client)
	return route
}

func (route *Route) AddClient(client *Client) {
	route.mutex.Lock()
	defer route.mutex.Unlock()
	if _, ok := route.clients[client.uid]; ok {
		log.Info("client exists")
	}
	route.clients[client.uid] = client
}

func (route *Route) RemoveClient(client *Client) {
	route.mutex.Lock()
	defer route.mutex.Unlock()
	if _, ok := route.clients[client.uid]; ok {
		if route.clients[client.uid] == client {
			delete(route.clients, client.uid)
			return
		}
	}
	log.Info("client non exists")
}

func (route *Route) FindClient(uid int64) *Client {
	route.mutex.Lock()
	defer route.mutex.Unlock()

	c, ok := route.clients[uid]
	if ok {
		return c
	} else {
		return nil
	}
}

func (route *Route) GetClientUids() map[int64]int32 {
	route.mutex.Lock()
	defer route.mutex.Unlock()
	uids := make(map[int64]int32)
	for uid, c := range route.clients {
		uids[uid] = int32(c.tm.Unix())
	}
	return uids
}

