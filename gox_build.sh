#!/bin/bash

mkdir build -p
pushd build

gox ../...

for f in darwin_386 darwin_amd64 freebsd_386 freebsd_amd64 freebsd_arm linux_386 linux_amd64 linux_arm netbsd_386 netbsd_amd64 netbsd_arm openbsd_386 openbsd_amd64 plan9_386;
do 
	mkdir $f
	mv getpan_$f $f/getpan
	mv mirropan_$f $f/mirropan
	mv pandex_$f $f/pandex
	mv smartpan_$f $f/smartpan
done

for f in windows_386 windows_amd64
do
	mkdir $f
	mv getpan_$f.exe $f/getpan.exe
	mv mirropan_$f.exe $f/mirropan.exe
	mv pandex_$f.exe $f/pandex.exe
	mv smartpan_$f.exe $f/smartpan.exe
done

for f in darwin_386 darwin_amd64 freebsd_386 freebsd_amd64 freebsd_arm linux_386 linux_amd64 linux_arm netbsd_386 netbsd_amd64 netbsd_arm openbsd_386 openbsd_amd64 plan9_386;
do
	pushd $f
	tar -zcf ../gopan-0.3a-$f.tar.gz ./*
	popd
done

#for f in windows_386 windows_amd64
#do
#	pushd $f
#	zip ./* ../$f-0.3.zip
#	popd
#done

popd build
