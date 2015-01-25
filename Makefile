all:voip

voip:im.go client.go route.go protocol.go  set.go  config.go tunnel.go
	go build -o voip im.go client.go route.go protocol.go  set.go  config.go tunnel.go

install:all
	cp voip ./bin
clean:
	rm -f voip
