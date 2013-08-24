rosella: main.go rosella.go
	go build -o ircd main.go rosella.go server.go client.go

clean:
	rm ircd

