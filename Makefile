.PHONY: sqlc test vet build web
sqlc:
	sqlc generate
vet:
	go vet ./...
test:
	go test ./...
web:
	npm --prefix web run build
build: web
	go build -o bin/easypos ./cmd/easypos
