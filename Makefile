rosella: main.go rosella.go
	go build -o ircd main.go rosella.go

clean:
	rm ircd

