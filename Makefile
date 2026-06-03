# Go toolchain path on this system
GO ?= go
BINARY = gobungle
SOUNDTEST = soundtest
LOGFILE = gobungle.log

.PHONY: all build run clean log help soundtest

all: build

build:
	$(GO) build -o $(BINARY) ./cmd/gobungle

soundtest:
	$(GO) build -o $(SOUNDTEST) ./cmd/soundtest

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)
	rm -f $(LOGFILE)

log:
	tail -f $(LOGFILE)

help:
	@echo "Available Makefile targets:"
	@echo "  make build  - Compile the helicopter flight simulator binary"
	@echo "  make run    - Compile and launch the game"
	@echo "  make clean  - Remove the compiled binary and log file"
	@echo "  make log    - Tail the structured slog log file live"
	@echo "  make help   - Show this help summary"
