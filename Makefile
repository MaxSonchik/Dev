# DevOS Global Makefile

# Configuration
DIST_DIR := dist
GO_TOOLS := d-env d-guard d-ci d-recon d-top
RUST_TOOLS := d-shark
RUST_SECURITY := d-ransom d-paladin

.PHONY: all clean build-go build-rust test release

all: clean build-go build-rust

# --- BUILD STEPS ---

build-go:
	@echo "🚀 Building Go Tools..."
	@mkdir -p $(DIST_DIR)
	@for tool in $(GO_TOOLS); do \
		echo "  -> Building $$tool..."; \
		cd tools/$$tool && go mod tidy && go build -o ../../$(DIST_DIR)/$$tool cmd/$$tool/main.go && cd ../..; \
	done

build-rust:
	@echo "🦀 Building Rust Tools..."
	@mkdir -p $(DIST_DIR)
	
	# Build d-shark (standalone tool)
	@echo "  -> Building d-shark..."
	@cd tools/d-shark && cargo build --release && cp target/release/d-shark ../../$(DIST_DIR)/
	
	# Build Security Suite (workspace)
	@echo "  -> Building Security Suite (d-ransom, d-paladin)..."
	@cd devo-security && cargo build --release
	@cp devo-security/target/release/d-ransom $(DIST_DIR)/
	@cp devo-security/target/release/d-paladin $(DIST_DIR)/

# --- CLEAN ---

clean:
	@echo "🧹 Cleaning artifacts..."
	@rm -rf $(DIST_DIR)
	@rm -rf tools/*/target
	@rm -rf devo-security/target

# --- TESTS ---

test:
	@echo "🧪 Running Tests..."
	@for tool in $(GO_TOOLS); do \
		echo "  -> Testing $$tool..."; \
		cd tools/$$tool && go test ./... && cd ../..; \
	done
	@cd tools/d-shark && cargo test
	@cd devo-security && cargo test
