.PHONY: all gen 

# Default target
all: gen

# Code generation target
gen:
	@echo "Generating code..."
	buf generate