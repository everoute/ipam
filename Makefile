.PHONY: image-generate generate docker-generate test docker-test publish

CONTROLLER_GEN=$(shell which controller-gen)

image-generate:
	docker build -f build/image/generate/Dockerfile -t localhost/generate ./build/image/generate/

image-test:
	docker build -f build/image/unit-test/Dockerfile -t localhost/unit-test ./build/image/unit-test/

generate: manifests codegen prefix

codegen:
	deepcopy-gen -O zz_generated.deepcopy --go-header-file ./hack/boilerplate.generatego.txt --input-dirs ./api/ipam/...

docker-generate: image-generate
	$(eval WORKDIR := /go/src/github.com/everoute/ipam)
	docker run --rm -iu 0:0 -w $(WORKDIR) -v $(CURDIR):$(WORKDIR) localhost/generate make generate

test:
	go test ./... -p 1 --race --coverprofile coverage.out

docker-test: image-test
	$(eval WORKDIR := /go/src/github.com/everoute/ipam)
	docker run --rm -iu 0:0 -w $(WORKDIR) -v $(CURDIR):$(WORKDIR) localhost/unit-test bash

prefix:
	find . -name "*.go" -exec gci write --Section Standard --Section Default --Section "Prefix(github.com/everoute/ipam)" {} +

manifests:
	$(CONTROLLER_GEN) crd paths="./api/..." output:crd:dir=deploy/crds output:stdout

publish:
