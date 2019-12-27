build: *.go
	mkdir -p bin
	go build -o bin/whalewatcher .

test:
	go test ./...

docker:
	if ! which -s docker; then echo "Docker not installed"; exit 1; fi
	docker build -t whalewatcher:latest .

demo: docker
	for TOOL in jq curl docker-compose; do if ! which -s "$$TOOL"; then echo "$$TOOL not installed"; exit 1; fi; done
	docker-compose run --rm -d demo_monitor && sleep 3 && watch -n 3 'curl -sS http://localhost:4444 | jq .'

clean:
	rm -rf bin
	docker images -a --format '{{.Repository}} {{.ID}}' | grep --color=auto 'whalewatcher' | cut -d ' ' -f2 | xargs docker rmi -f
