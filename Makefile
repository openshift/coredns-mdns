.DEFAULT_GOAL:=help
GO_TEST_FLAGS = $(VERBOSE)

.PHONY: fmt
fmt: ## Run go fmt against code
	go fmt -mod=mod *.go
	git diff --exit-code

.PHONY: vet
vet: ## Run go vet against code
	go vet *.go

.PHONY: test
test: ## Run go test against code
	go test -mod=mod -v $(GO_TEST_FLAGS) ./

.PHONY: build-coredns
build-coredns: ## Build coredns using the local branch of coredns-mdns
	hack/build-coredns.sh
