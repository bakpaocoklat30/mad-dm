.PHONY: all build build-all clean run-cli run-server

all: build

build:
	@mkdir -p bin
	@cd engine && go build -o ../bin/mad-dm ./cmd/cli
	@cd engine && go build -o ../bin/mad-dm-server ./cmd/server
	@echo "Build completed: bin/mad-dm and bin/mad-dm-server"

build-all:
	@chmod +x scripts/build.sh
	@./scripts/build.sh

clean:
	@rm -rf bin/
	@echo "Cleaned bin/"
