.PHONY: image-generate generate docker-generate test docker-test publish

CONTROLLER_GEN=$(shell which controller-gen)

image-generate:
	docker build -f build/image/generate/Dockerfile -t localhost/generate ./build/image/generate/

generate: prefix manifests
	

docker-generate: image-generate
	$(eval WORKDIR := /go/src/github.com/everoute/ipam)
	docker run --rm -iu 0:0 -w $(WORKDIR) -v $(CURDIR):$(WORKDIR) localhost/generate make generate

test:
	go test ./... --race --coverprofile coverage.out

docker-test:
	$(eval WORKDIR := /go/src/github.com/everoute/ipam)
	docker run --rm -iu 0:0 -w $(WORKDIR) -v $(CURDIR):$(WORKDIR) golang:1.20 make test

prefix:
	find . -name "*.go" -exec gci write --Section Standard --Section Default --Section "Prefix(github.com/everoute/ipam)" {} +

manifests:
	$(CONTROLLER_GEN) crd paths="./api/..." output:crd:dir=deploy/crds output:stdout

publish:
