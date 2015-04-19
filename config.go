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

import "strconv"
import "log"
import "github.com/jimlawless/cfg"


type Config struct {
	port               int
	tunnel_port        int
	tunnel_port_v2     int
	redis_address      string
}

func get_int(app_cfg map[string]string, key string) int {
	concurrency, present := app_cfg[key]
	if !present {
		log.Fatalf("key:%s non exist", key)
	}
	n, err := strconv.Atoi(concurrency)
	if err != nil {
		log.Fatalf("key:%s is't integer", key)
	}
	return n
}

func get_string(app_cfg map[string]string, key string) string {
	concurrency, present := app_cfg[key]
	if !present {
		log.Fatalf("key:%s non exist", key)
	}
	return concurrency
}

func get_opt_string(app_cfg map[string]string, key string) string {
	concurrency, present := app_cfg[key]
	if !present {
		return ""
	}
	return concurrency
}

func read_cfg(cfg_path string) *Config {
	config := new(Config)
	app_cfg := make(map[string]string)
	err := cfg.Load(cfg_path, app_cfg)
	if err != nil {
		log.Fatal(err)
	}

	config.port = get_int(app_cfg, "port")
	config.tunnel_port = get_int(app_cfg, "tunnel_port")
	config.tunnel_port_v2 = get_int(app_cfg, "tunnel_port_v2")
	config.redis_address = get_string(app_cfg, "redis_address")
	return config
}
