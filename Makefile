build: *.go
	mkdir -p bin
	go build -o bin/whalewatcher .

clean:
	rm -rf bin
