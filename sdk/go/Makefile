# Go SDK maintenance targets. Run from sdk/go/.

SCHEMA_SRC := ../../schemas/v0.1
SCHEMA_DST := vwp/schema/vwpschemas/v0.1

.PHONY: all help test vet sync-schemas

all: help

help:
	@echo "Targets:"
	@echo "  test          go vet + go test ./..."
	@echo "  sync-schemas  mirror $(SCHEMA_SRC) into the embedded copy at $(SCHEMA_DST)"
	@echo "                (the schema drift test fails go test until this is run after a schema change)"

test: vet
	go test ./...

vet:
	go vet ./...

# Mirror the canonical schema tree into the go:embed directory: a clean
# mirror (removed files are deleted), *.json only. The drift test in
# vwp/schema enforces that this stays in sync.
sync-schemas:
	rsync -a --delete --include='*/' --include='*.json' --exclude='*' \
		$(SCHEMA_SRC)/ $(SCHEMA_DST)/
	@echo "embedded schemas synced from $(SCHEMA_SRC)"
