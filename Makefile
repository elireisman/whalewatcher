build: *.go
	mkdir -p bin
	go build -o bin/whalewatcher .

docker:
	if ! which -s docker; then echo "Docker not installed"; exit 1; fi
	docker build -t whalewatcher:latest .

example: docker
	if ! which -s docker-compose; then echo "Docker Compose not installed"; exit 1; fi
	docker-compose run whalewatcher-demo

clean:
	rm -rf bin
