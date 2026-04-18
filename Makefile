# CS Stats Tracker Build System
#
# Usage: make <target> [VAR=val]
#
# Variables:
#   BIN_DIR    Output directory for binaries (default: bin)
#   BINARY     Binary name (default: csstatstracker, .exe appended on Windows)

# -- Platform detection -------------------------------------------------------

UNAME_S := $(shell uname -s)
IS_WIN  := $(findstring MINGW,$(UNAME_S))$(findstring MSYS,$(UNAME_S))$(findstring CYGWIN,$(UNAME_S))

# -- Common variables ---------------------------------------------------------

BIN_DIR     := bin
BIN_NAME    := csstatstracker
ifdef IS_WIN
  BINARY    := $(BIN_DIR)/$(BIN_NAME).exe
else
  BINARY    := $(BIN_DIR)/$(BIN_NAME)
endif

GO          := go
CGO_ENABLED := 1

# Windows: locate a MinGW gcc (CGO needs it for sqlite + fyne) and inject its
# directory into PATH in Windows-native form so go.exe picks it up even when
# invoked from Git Bash via ezwinports make.
ifdef IS_WIN
  MINGW_CANDIDATES := \
    C:/msys64/mingw64/bin \
    C:/msys64/ucrt64/bin \
    C:/mingw64/bin \
    C:/TDM-GCC-64/bin
  MINGW_BIN := $(firstword $(foreach d,$(MINGW_CANDIDATES),$(if $(wildcard $(d)/gcc.exe),$(d))))
  ifeq ($(MINGW_BIN),)
    $(warning No MinGW gcc found. Install MSYS2 mingw-w64-gcc at C:/msys64/mingw64 or set PATH manually.)
  else
    MINGW_BIN_WIN := $(subst /,\,$(MINGW_BIN))
    export PATH := $(MINGW_BIN_WIN);$(PATH)
  endif
endif

.PHONY: all build build-dev run test lint fmt tidy vet clean help

all: build

# -- Build --------------------------------------------------------------------
#
# On Windows we use `fyne package` so the produced .exe gets the icon resource
# and the windowsgui subsystem flag (no stray console window). On other OSes,
# plain `go build` is fine — Fyne reads Icon.png at runtime.

ifdef IS_WIN

FYNE := $(shell command -v fyne 2>/dev/null)

build:
	@echo "==> Building $(BIN_NAME) (windows, packaged)..."
	@mkdir -p $(BIN_DIR)
ifeq ($(FYNE),)
	@echo "    Installing fyne CLI..."
	$(GO) install fyne.io/tools/cmd/fyne@latest
endif
	CGO_ENABLED=$(CGO_ENABLED) fyne package -os windows -src ./cmd -icon $(CURDIR)/Icon.png -name $(BIN_NAME)
	@mv -f cmd/$(BIN_NAME).exe $(BINARY) 2>/dev/null || mv -f $(BIN_NAME).exe $(BINARY)
	@echo "    Built: $(BINARY)"

# Faster iteration build without packaging (console visible, no embedded icon).
build-dev:
	@echo "==> Building $(BIN_NAME) (dev, no packaging)..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build -o $(BINARY) ./cmd
	@echo "    Built: $(BINARY)"

else

build:
	@echo "==> Building $(BIN_NAME)..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build -o $(BINARY) ./cmd
	@echo "    Built: $(BINARY)"

build-dev: build

endif

# -- Run ----------------------------------------------------------------------

run: build
	@echo "==> Starting $(BIN_NAME)..."
	./$(BINARY)

# -- Tests --------------------------------------------------------------------

test:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test ./... -count=1

# -- Lint / format / vet ------------------------------------------------------

lint:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run --timeout 2m ./...

fmt:
	gofmt -s -w .

vet:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) vet ./...

tidy:
	$(GO) mod tidy

# -- Clean --------------------------------------------------------------------

clean:
	rm -rf $(BIN_DIR)
	@echo "Cleaned."

# -- Help ---------------------------------------------------------------------

help:
	@echo ""
	@echo "  CS Stats Tracker Build System"
	@echo "  ============================="
	@echo ""
	@echo "  Usage: make <target>"
	@echo ""
	@echo "  Targets:"
	@echo "    build      Compile the binary to $(BIN_DIR)/ (packaged with icon on Windows)"
	@echo "    build-dev  Plain go build (Windows: skips fyne packaging)"
	@echo "    run        Build and run the app"
	@echo "    test       Run unit tests"
	@echo "    lint       Run golangci-lint"
	@echo "    vet        Run go vet"
	@echo "    fmt        Format Go source with gofmt -s"
	@echo "    tidy       Tidy go.mod"
	@echo "    clean      Remove $(BIN_DIR)/"
	@echo ""
