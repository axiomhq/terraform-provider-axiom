TEST?=$$(go list ./... | grep -v 'vendor')
HOSTNAME=locally
NAMESPACE=debug
NAME=axiom
BINARY=terraform-provider-${NAME}
GIT_TAG:=$(shell git describe --tags --exact-match 2>/dev/null)
VERSION?=$(if $(GIT_TAG),$(patsubst v%,%,$(GIT_TAG)),dev)
LDFLAGS=-X terraform-provider-axiom-provider/axiom.providerVersion=${VERSION}
OS_ARCH=darwin_arm64

default: install

build:
	go build -ldflags "${LDFLAGS}" -o ${BINARY}

release:
	GOOS=darwin GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o ./bin/${BINARY}_${VERSION}_darwin_amd64
	GOOS=freebsd GOARCH=386 go build -ldflags "${LDFLAGS}" -o ./bin/${BINARY}_${VERSION}_freebsd_386
	GOOS=freebsd GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o ./bin/${BINARY}_${VERSION}_freebsd_amd64
	GOOS=freebsd GOARCH=arm go build -ldflags "${LDFLAGS}" -o ./bin/${BINARY}_${VERSION}_freebsd_arm
	GOOS=linux GOARCH=386 go build -ldflags "${LDFLAGS}" -o ./bin/${BINARY}_${VERSION}_linux_386
	GOOS=linux GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o ./bin/${BINARY}_${VERSION}_linux_amd64
	GOOS=linux GOARCH=arm go build -ldflags "${LDFLAGS}" -o ./bin/${BINARY}_${VERSION}_linux_arm
	GOOS=openbsd GOARCH=386 go build -ldflags "${LDFLAGS}" -o ./bin/${BINARY}_${VERSION}_openbsd_386
	GOOS=openbsd GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o ./bin/${BINARY}_${VERSION}_openbsd_amd64
	GOOS=solaris GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o ./bin/${BINARY}_${VERSION}_solaris_amd64
	GOOS=windows GOARCH=386 go build -ldflags "${LDFLAGS}" -o ./bin/${BINARY}_${VERSION}_windows_386
	GOOS=windows GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o ./bin/${BINARY}_${VERSION}_windows_amd64

generate:
	go generate ./...

install: build
	mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}
	mv ${BINARY} ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}

test:
	go test -count=1 -parallel=4 ./...

testacc:
	TF_ACC=1 go test -count=1 -parallel=4 -timeout 10m -v ./...
