GO := go
NAME := tibench
LOCAL_BIN := bin/$(NAME)

ifndef $(GOPATH)
    GOPATH=$(shell go env GOPATH)
    export GOPATH
endif
BINDIR=$(GOPATH)/bin

.PHONY: install clean

install:
	$(GO) build -v -o $(LOCAL_BIN) ./main.go
	@echo "install -m 755 $(LOCAL_BIN) $(BINDIR)"
	install -m 755 $(LOCAL_BIN) $(BINDIR)

clean:
	$(GO) clean -i
	rm -rf bin/*
