#!/bin/bash
rm getpan/getpan/perl_core.go
echo "package main" >> getpan/getpan/perl_core.go
echo "" >> getpan/getpan/perl_core.go
echo "var perl_core = map[string]int{" >> getpan/getpan/perl_core.go
for i in {A..Z}; do 
	echo "Loading modules: $i"
	LETTER=$i perl -Mojo -MData::Dumper -e 'g("http://perldoc.perl.org/index-modules-" . $ENV{"LETTER"} . ".html")->dom->find("html")->find("body")->find("#page")->find("#content_body")->find("ul")->find("a")->each(sub{@n = split "\n", eval{shift->text}; print "\"" . $_ . "\": 1,\n" for @n; print "\n"})' >> getpan/getpan/perl_core.go
done
echo "Loading pragmas"
LETTER=$i perl -Mojo -MData::Dumper -e 'g("http://perldoc.perl.org/index-pragmas.html")->dom->find("html")->find("body")->find("#page")->find("#content_body")->find("ul")->find("a")->each(sub{@n = split "\n", eval{shift->text}; print "\"" . $_ . "\": 1,\n" for @n; print "\n"})' >> getpan/getpan/perl_core.go
echo '"perl": 1,' >> getpan/getpan/perl_core.go
echo "}" >> getpan/getpan/perl_core.go
echo "" >> getpan/getpan/perl_core.go
