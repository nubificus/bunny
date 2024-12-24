# Copyright (c) 2023-2024, Nubificus LTD
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Versioning variables
COMMIT         := $(shell git describe --dirty --long --always)
VERSION        := $(shell cat $(CURDIR)/VERSION)-$(COMMIT)

# Path variables
#
# Use absolute paths just for sanity.
#? BUILD_DIR Directory to place produced binaries (default: ${CWD}/dist)
BUILD_DIR      ?= ${CURDIR}/dist
VENDOR_DIR     := ${CURDIR}/vendor
#? PREFIX Directory to install urunc and shim (default: /usr/local/bin)
PREFIX         ?= /usr/local/bin

# Binary variables
PUN_BIN        := $(BUILD_DIR)/pun

# Golang variables
#? GO go binary to use (default: go)
GO             ?= go
GO_FLAGS       := GOOS=linux
GO_FLAGS       += CGO_ENABLED=0

# Linking variables
LDFLAGS_COMMON := -X main.version=$(VERSION)
LDFLAGS_STATIC := --extldflags -static
LDFLAGS_OPT    := -s -w

#? CNTR_TOOL Tool to run the linter container (default: docker)
CNTR_TOOL ?= docker
CNTR_OPTS ?= run --rm -it

# Linking variables
LINT_CNTR_OPTS ?= $(CNTR_OPTS) -v $(CURDIR):/app -w /app
#? LINT_CNTR_IMG The linter image to use (default: golangci/golangci-lint:v1.53.3)
LINT_CNTR_IMG  ?= golangci/golangci-lint:v1.53.3
LINT_CNTR_CMD  ?= golangci-lint run -v --timeout=5m

# Main Building rules
#
# By default we opt to build static binaries targeting the host archiotecture.
# However, we build shim as a dynamically-linked binary.

## default Build shim and urunc statically for host arch.(default).
.PHONY: default
default: $(PUN_BIN)

# Just an alias for $(VENDOR_DIR) for easie invocation
## prepare Run go mod vendor and veridy.
prepare: $(VENDOR_DIR)

# Add tidy as order-only prerequisite. In that way, since tidy does not
# produce any file and executes all the time, we avoid the execution
# of $(VENDOR_DIR) rule, if the file already exists
$(VENDOR_DIR):
	$(GO) mod tidy
	$(GO) mod vendor
	$(GO) mod verify

# Add tidy and as order-only prerequisite. In that way, since tidy and
# vendor do notproduce any file and execute all the time,
# we avoid the rebuilding of urunc if it has previously built and the
# source files have not changed.
$(PUN_BIN): main.go | prepare
	$(GO_FLAGS) $(GO) build \
		-ldflags "$(LDFLAGS_COMMON) $(LDFLAGS_STATIC) $(LDFLAGS_OPT)" \
		-o $(PUN_BIN)

## install Install urunc and shim in PREFIX
.PHONY: install
install: $(PUN_BIN)
	install -D -m0755 $^ $(PREFIX)/pun

## uninstall Remove urunc and shim from PREFIX
.PHONY: uninstall
uninstall:
	rm -f $(PREFIX)/pun

## distclean Remove build and vendor directories
.PHONY: distclean
distclean: clean
	rm -fr $(VENDOR_DIR)

## clean build directory
.PHONY: clean
clean:
	rm -fr $(BUILD_DIR)

# Linting targets
## lint Run the lint test using a golang container
.PHONY: lint
lint:
	$(CNTR_TOOL) $(LINT_CNTR_OPTS) $(LINT_CNTR_IMG) $(LINT_CNTR_CMD)

## help Show this help message
help:
	@echo 'Usage: make <target> <flags>'
	@echo 'Targets:'
	@grep -w "^##" $(MAKEFILE_LIST) | sed -n 's/^## /\t/p' | sed -n 's/ /\@/p' | column -s '\@' -t
	@echo 'Flags:'
	@grep -w "^#?" $(MAKEFILE_LIST) | sed -n 's/^#? /\t/p' | sed -n 's/ /\@/p' | column -s '\@' -t
