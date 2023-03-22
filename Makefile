.PHONY: test
test: ## Run tests.
	go test ./... -coverprofile cover.out -v