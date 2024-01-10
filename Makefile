all: generate init tidy build test

generate:
	cd test/proto; npx buf mod update; cd ..; npx buf generate; cd ..

init:
	go mod init go.goms.io/aks/rp/aks-middleware

tidy:
	go mod tidy

build:
	go build ./...

test:
	go test ./...