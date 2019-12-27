build: *.go
	@mkdir -p bin
	go build -o bin/whalewatcher .

test:
	go test ./...

docker:
	if ! which -s docker; then echo "Docker not installed"; exit 1; fi
	docker build -t whalewatcher:latest .

example: docker
	if ! which -s docker-compose; then echo "docker-compose not installed"; exit 1; fi
	./script/exec_in_make whalewatcher

demo: docker
	if ! which -s docker-compose; then echo "docker-compose not installed"; exit 1; fi
	./script/exec_in_make awaiting_warmup

clean:
	@rm -rf bin
	@docker-compose down -v
	@docker images -a --format '{{.Repository}} {{.ID}}' | grep whalewatcher | cut -d ' ' -f2 | xargs docker rmi -f
