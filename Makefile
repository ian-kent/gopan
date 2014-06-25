all: deps install

deps:
	go get github.com/ian-kent/gotcha/...
	go get github.com/ian-kent/go-log/...
	go get github.com/mitchellh/gox
	go get code.google.com/p/go.net/html
	go get gopkg.in/yaml.v1

install: pandex mirropan getpan smartpan

dist: smartpan
	rm ./build -rf
	./gox_build.sh "0.4b"

pandex:
	go install ./pandex

mirropan:
	go install ./mirropan

getpan:
	go install ./getpan

smartpan:
	cd smartpan && make # to compile assets

.PHONY: all deps install build dist pandex mirropan getpan smartpan
