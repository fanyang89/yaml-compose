.PHONY: build
build:
	go build -o yaml-compose ./main.go

.PHONY: test
test:
	go test ./...

.PHONY: install
install: build
	go install .
