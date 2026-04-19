BINARY_NAME=ev2md
BUILD_DIR=build
BUILD_DATE=$(shell date '+%Y-%m-%d')

PLATFORMS = \
	"linux amd64" \
	"darwin amd64" \
	"darwin arm64" \
	"windows amd64"

.PHONY: all clean build install

all: clean build

build:
	@echo "Building $(BINARY_NAME) for multiple platforms..."
	@for PLATFORM in $(PLATFORMS); do \
		OS=$$(echo $$PLATFORM | awk '{print $$1}'); \
		ARCH=$$(echo $$PLATFORM | awk '{print $$2}'); \
		EXT=""; \
		if [ "$$OS" = "windows" ]; then EXT=".exe"; fi; \
		OUTDIR=$(BUILD_DIR)/$${OS}_$${ARCH}; \
		OUTFILE=$$OUTDIR/$(BINARY_NAME)$$EXT; \
		echo " > $$OUTFILE"; \
		mkdir -p $$OUTDIR; \
		GOOS=$$OS GOARCH=$$ARCH go build -ldflags "-X main.buildDate=$(BUILD_DATE)" -o $$OUTFILE . ; \
	done

install:
	@echo "Installing $(BINARY_NAME) to ~/bin..."
	@OS=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	ARCH=$$(uname -m); \
	if [ "$$ARCH" = "x86_64" ]; then ARCH=amd64; fi; \
	SRC=$(BUILD_DIR)/$${OS}_$${ARCH}/$(BINARY_NAME); \
	DEST=$$HOME/bin/$(BINARY_NAME); \
	if [ ! -f "$$SRC" ]; then \
		echo "Built binary not found: $$SRC"; \
		exit 1; \
	fi; \
	mkdir -p "$$HOME/bin"; \
	cp "$$SRC" "$$DEST"; \
	chmod +x "$$DEST"; \
	echo "Installed to $$DEST"

clean:
	@echo "Cleaning build directory..."
	@rm -rf $(BUILD_DIR)
