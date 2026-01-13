.PHONY: all gen dev up down db-gen

# Default target
all: gen

# Code generation target
gen:
	@echo "Generating code..."
	buf generate