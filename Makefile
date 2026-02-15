.PHONY: build
build:
	go build -o yaml-compose ./main.go

.PHONY: install
install: build
	mv yaml-compose $(GOPATH)/bin/
