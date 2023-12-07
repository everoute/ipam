.PHONY: image-generate generate docker-generate test docker-test publish

image-generate:
	docker build -f build/image/generate/Dockerfile -t localhost/generate ./build/image/generate/

generate:
	find . -name "*.go" -exec gci write --Section Standard --Section Default --Section "Prefix(github.com/everoute/template-repo)" {} +

docker-generate: image-generate
	$(eval WORKDIR := /go/src/github.com/everoute/template-repo)
	docker run --rm -iu 0:0 -w $(WORKDIR) -v $(CURDIR):$(WORKDIR) localhost/generate make generate

test:
	go test ./... --race --coverprofile coverage.out

docker-test:
	$(eval WORKDIR := /go/src/github.com/everoute/template-repo)
	docker run --rm -iu 0:0 -w $(WORKDIR) -v $(CURDIR):$(WORKDIR) golang:1.19 make test

publish:
