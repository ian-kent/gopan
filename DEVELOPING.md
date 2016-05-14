Developing GoPAN
================

- [Setup your Go environment](http://golang.org/doc/install)
- [Finish setting up your Go environment](http://golang.org/doc/code.html)
- Clone [GoPAN](git@github.com:companieshouse/gopan.git) into `$GOHOME/src/github.com/companieshouse/gopan`
- Run `make deps` to install GoPAN dependencies
- Run `make` to build and install GoPAN (to $GOHOME/bin)
- Run `make dist` to build native binaries
  - Configure [gox](https://github.com/mitchellh/gox) first, e.g. running `gox -build-toolchain`

### Go libraries

Although GoPAN contains four tools (SmartPAN, GetPAN, MirroPAN and PANdex), GetPAN
and PANdex are also independent Go libraries.

| Library                        | Description
| ------------------------------ | ------------------------------------------------------
| [getpan/getpan](getpan/getpan) | Resolves cpanfile and module dependencies
| [pandex/pandex](pandex/pandex) | Indexes modules and packages in a gopancache directory

### The index file

The index file behaves more like a transaction log.

As the index changes in-memory, those changes are written to the log.

Removals can be written by prefixing the line (after the initial indent) with a `-`.

The index file can be flattened using the `-flatten` option in [PANdex](../pandex/README.md)

### get_perl_core.sh

If you want to change the version of Perl supported by GoPAN, run `./get_perl_core.sh`
to update `getpan/getpan/perl_core.go`.

You'll need Mojolicious and Perl installed.

It is currently built against Perl 5.18.2.

`perl_core.go` is used to exclude Perl core modules from the dependency tree built
by GetPAN (and SmartPAN for imports).

This prevents GetPAN from updating Perl core modules (or attempting to install Perl itself!)

See [getpan/Carton.md](getpan/Carton.md) for more information.

### Pull requests

Before submitting a pull request, please format your code:

    go fmt ./...

## Licence

Copyright ©‎ 2014, Ian Kent (http://www.iankent.eu).

Released under MIT license, see [LICENSE](LICENSE.md) for details.
