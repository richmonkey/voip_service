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

import "sync"
import log "github.com/golang/glog"


type Route struct {
	appid  int64
	mutex   sync.Mutex
	clients map[int64]ClientSet
}

func NewRoute(appid int64) *Route {
	route := new(Route)
	route.appid = appid
	route.clients = make(map[int64]ClientSet)
	return route
}

func (route *Route) AddClient(client *Client) {
	route.mutex.Lock()
	defer route.mutex.Unlock()
	set, ok := route.clients[client.uid]; 
	if !ok {
		set = NewClientSet()
		route.clients[client.uid] = set
	}
	set.Add(client)
}

func (route *Route) RemoveClient(client *Client) bool {
	route.mutex.Lock()
	defer route.mutex.Unlock()
	if set, ok := route.clients[client.uid]; ok {
		set.Remove(client)
		if set.Count() == 0 {
			delete(route.clients, client.uid)
		}
		return true
	}
	log.Info("client non exists")
	return false
}

func (route *Route) FindClientSet(uid int64) ClientSet {
	route.mutex.Lock()
	defer route.mutex.Unlock()

	set, ok := route.clients[uid]
	if ok {
		return set.Clone()
	} else {
		return nil
	}
}

func (route *Route) GetClientUids() map[int64]int32 {
	return nil
	// route.mutex.Lock()
	// defer route.mutex.Unlock()
	// uids := make(map[int64]int32)
	// for uid, c := range route.clients {
	// 	uids[uid] = int32(c.tm.Unix())
	// }
	// return uids
}

