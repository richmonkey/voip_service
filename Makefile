all:voip

voip:voip.go client.go route.go protocol.go  set.go  config.go tunnel.go user.go app_route.go
	go build -o voip voip.go client.go route.go protocol.go  set.go  config.go tunnel.go user.go app_route.go

install:all
	cp voip ./bin
clean:
	rm -f voip
