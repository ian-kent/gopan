VERSION = 0.9

all: deps install fmt

fmt:
	go fmt ./...

deps:
	go get github.com/companieshouse/gotcha/...
	go get github.com/ian-kent/go-log/log
	go get github.com/mitchellh/gox
	go get gopkg.in/yaml.v1
	go get golang.org/x/net/html

install: pandex mirropan getpan smartpan

dist: smartpan
	rm -rf ./build
	./gox_build.sh "$(VERSION)"

pandex:
	go install ./pandex

mirropan:
	go install ./mirropan

getpan:
	go install ./getpan

smartpan:
	cd smartpan && make # to compile assets

.PHONY: all deps install build dist pandex mirropan getpan smartpan
