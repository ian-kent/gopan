all: deps install

deps:
	go get github.com/ian-kent/gotcha/gotcha
	go get github.com/ian-kent/go-log/log

install:
	go install ./pandex
	go install ./mirropan
	go install ./getpan
	cd smartpan && make

build:
	go build ./pandex
	go build ./mirropan
	go build ./getpan
	cd smartpan && make

dist:
	cd smartpan && make # to compile assets
	rm ./build -rf
	./gox_build.sh "0.3b"

.PHONY: all deps install build dist
