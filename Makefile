BIN="./bin"
MOD="github.com/david-wiles/crawl-project"
MAIN="cmd/main.go"
EXE="crawl-project"

all: clean fmt build

clean:
	rm $(BIN)/$(EXE)

build:
	go build -o $(BIN)/$(EXE) $(MAIN)

fmt:
	goimports -w *.go
