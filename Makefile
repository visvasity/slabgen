# Copyright (c) 2025 Visvasity LLC

export GO ?= $(shell which go)
export GOBIN = $(CURDIR)
export PATH = $(CURDIR):$(HOME)/bin:$(HOME)/go/bin:/bin:/usr/bin:/usr/local/bin:/sbin:/usr/sbin
export GOTESTFLAGS ?=

.PHONY: all
all:
	$(GO) build ./input
	rm -rf ./output/*.slab.go
	$(GO) run . -inpkg=./input -outpkg=output -outdir=./output TestFinalSliceKind1 TestFinalSliceKind2 TestFinalSliceKind3 TestFinalSliceKind4 TestFinalSliceKind5 TestFinalSliceKind6 TestFinalSliceKind7 TestFinalSliceKind8
	$(GO) run . -inpkg=./input -outpkg=output -outdir=./output TestGenericBlock TestPair TestMixedPair
	$(GO) run . -inpkg=./input -outpkg=output -outdir=./output SuperBlock
	$(GO) run . -inpkg=./input -outpkg=output -outdir=./output DataBlock
	$(GO) build ./output
	goimports -w ./output/*.go || true
	$(GO)	test -fullpath -failfast -count=1 -coverprofile=coverage.out $(GOTESTFLAGS) -v ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

.PHONY: clean
clean:
	git clean -f -X
