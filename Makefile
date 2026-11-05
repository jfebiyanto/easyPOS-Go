TESTDB ?= postgres://easypos:easypos@localhost:5432/easypos_test?sslmode=disable
.PHONY: run test build
run:
	go run ./cmd/easypos serve
build:
	go build -o bin/easypos ./cmd/easypos
test:
	EASYPOS_TEST_DATABASE_URL='$(TESTDB)' go test ./...
