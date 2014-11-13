GoPAN  [ ![Download](https://api.bintray.com/packages/ian-kent/generic/gopan/images/download.svg) ](https://bintray.com/ian-kent/generic/gopan/_latestVersion)
=====

GoPAN provides a full end-to-end toolchain to mirror, index, host and install from
a CPAN, BackPAN or DarkPAN repository.

Mirrors hosted by [SmartPAN](smartpan/README.md) are fully Carton/cpanm compatible,
but also support the `SmartPAN` interface used by [GetPAN](getpan/README.md).

[GetPAN](getpan/README.md) is also Carton/cpanm compatible and works with CPAN, BackPAN,
and DarkPAN mirrors using Pinto and Orepan. 

When used with [SmartPAN](smartpan/README.md),
the `SmartPAN` interface fixes dependency resolution issues which exist with typical CPAN 
indexes.

| Application                    | Description
| ------------------------------ | -----------------------------------------------
| [SmartPAN](smartpan/README.md) | Host a DarkPAN, BackPAN or CPAN mirror
| [GetPAN](getpan/README.md)     | Carton/cpanfile compatible dependency installer
| [MirroPAN](mirropan/README.md) | CPAN/BackPAN brute-force mirroring
| [PANdex](pandex/README.md)     | CPAN module indexer

## Getting started

### Requirements

You need Perl (preferably >=5.18.2) and the following modules for indexing
(used by PANDex and SmartPAN).

- Parse::LocalDistribution
- JSON::XS

Use getpan to simplify installation:

    getpan Parse::LocalDistribution JSON::XS
    getpan exec smartpan

All other GoPAN tools are entirely self-contained.

### Replacing Carton/cpanm

Use [GetPAN](getpan/README.md) in place of Carton or cpanm.

**Note** GetPAN uses cpanm internally, but dependency resolution is handled
by GetPAN to avoid versioning issues with CPAN indexes.

### Hosting a DarkPAN (or CPAN/BackPAN mirror)

Use [SmartPAN](smartpan/README.md) from any empty directory.

Make sure you have Perl, Parse::LocalDistribution and JSON::XS installed.

### Mirroring CPAN, BackPAN or any other DarkPAN index

Use [MirroPAN](mirropan/README.md) from any empty directory.

You'll need around 40GB of free space to mirror CPAN and BackPAN.

### Indexing a local mirror

Use [PANdex](pandex/README.md) to index any local CPAN, DarkPAN or BackPAN mirror.

This can be used to generate an index file for a mirror created with [MirroPAN](mirropan/README.md).

## Why?

It's faster. And it gets dependencies right.

It's also written in Go - so its highly portable with no installation!

## Contributing

See the [Developing GoPAN](DEVELOPING.md) guide.

## Licence

Copyright ©‎ 2014, Ian Kent (http://www.iankent.eu).

Released under MIT license, see [LICENSE](LICENSE.md) for details.
