SHELL := /bin/bash
SHELLFLAGS := -c
MAIN := $(shell pwd)


.ONESHELL:

run:
	@go run $(MAIN)/main.go


create-debug:
	@echo "Creting debug file"
	@go build -gcflags="all=-N -l" -o debugged ./main.go


.PHONY: create-debug 