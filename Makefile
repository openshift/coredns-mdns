.DEFAULT_GOAL:=help

.PHONY: fmt
fmt: ## Run go fmt against code
	go fmt *.go

.PHONY: vet
vet: ## Run go vet against code
	go vet *.go

.PHONY: build-coredns
build-coredns: ## Build coredns using the local branch of coredns-mdns
	hack/build-coredns.sh
