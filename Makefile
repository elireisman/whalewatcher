build: *.go
	@mkdir -p bin
	go build -o bin/whalewatcher .

test:
	go test ./...

docker:
	if ! which -s docker; then echo "Docker not installed"; exit 1; fi
	docker build -t whalewatcher:latest .

example:
	./script/example

demo:
	./script/demo

internal-demo:
	./script/internal-demo

push: docker
	docker login --username initialcontext docker.io
	docker tag whalewatcher:latest initialcontext/whalewatcher:latest
	docker push initialcontext/whalewatcher:latest

clean:
	@rm -rf bin
	@docker-compose down -v
	@docker images -a --format '{{.Repository}} {{.ID}}' | grep whalewatcher | cut -d ' ' -f2 | xargs docker rmi -f
