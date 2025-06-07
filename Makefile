


.ONESHELL:


create-debug:
	@echo "Creting debug file"
	@go build -gcflags="all=-N -l" -o debugged ./main.go

.PHONY: 