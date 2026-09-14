.PHONY: init
# init env
init:
	go install github.com/bufbuild/buf/cmd/buf@latest

.PHONY: api
# generate Go stubs and openapi.yaml
api:
	buf generate
	go mod tidy
