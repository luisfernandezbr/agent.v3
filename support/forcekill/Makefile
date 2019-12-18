TMPDIR := $(shell mktemp -d)

windows:
	@go build -o $(TMPDIR)/level1.exe level1/main.go
	@go build -o $(TMPDIR)/level2.exe level2/main.go
	@go build -o $(TMPDIR)/main.exe .
	@echo "Building main"
	@$(TMPDIR)/main

not-windows:
	@go build -o $(TMPDIR)/level1 level1/main.go
	@go build -o $(TMPDIR)/level2 level2/main.go
	@go build -o $(TMPDIR)/main .
	@echo "Building main"
	@$(TMPDIR)/main
