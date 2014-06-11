GoPAN Toolkit
=============

A collection of utilities for mirroring, hosting and indexing CPAN, BackPAN and DarkPAN repositories.

## Getting started

To save time, download the latest index and packages index:

- https://s3-eu-west-1.amazonaws.com/gopan/index.gz
- https://s3-eu-west-1.amazonaws.com/gopan/packages.gz

And gunzip these inside the cache directory (usually .gopancache)

Or you can create your own indexes:

- Be prepared to wait 3 days for a fully indexed CPAN/BackPAN mirror
- Run mirropan to build your CPAN/BackPAN mirror
- Run pandex to index your local mirror
  - You'll need to have JSON::XS installed for pandex to run.

Using the pre-built index files, you can use mirror readthrough instead
of hosting a full CPAN/BackPAN mirror.

- Run smartpan to host your local mirror
- Use getpan instead of Carton or cpanm

## Requirements

You need to have Perl (tested on 5.18.2) with these modules:

- Parse::LocalDistribution
- JSON::XS

## SmartPAN (smartpan)

CPAN repository server and web UI:

- Supports .tar.gz releases and cpanfile
- Import from other CPAN mirrors
- Resolve and import cpanfile dependencies
- Import from URL or local disk
- Upload with curl or a web browser

### HTTP API

You can query SmartPAN using HTTP to locate modules.

#### List all versions of a module

    curl -X GET http://path.to/SmartPAN/where/Module::Name

#### List all matching versions of a module

    curl -X GET http://path.to/SmartPAN/where/Module::Name/1.92
    curl -X GET http://path.to/SmartPAN/where/Module::Name/==3.99
    curl -X GET http://path.to/SmartPAN/where/Module::Name/>=2.00

#### TODO

- Add "latest" version to indexes for
- 02packages.details.txt(.gz) support

- CPAN/BackPAN readthrough
- Control versioning in mirror URL, e.g.
  - http://some.smartpan/with/Mojolicious/4.99/(authors/id/...)
  - http://some.smartpan/with/Mojolicious/>=5.00/(authors/id/...)

## MirroPAN (mirropan)

`mirropan` brute-force mirrors a *PAN (CPAN/BackPAN/DarkPAN) repository by scanning the HTML indexes for links.

This allows it to work behind a HTTP proxy.

It first indexes the authors, then packages, and finally downloads all packages to a local cache.

**WARNING** It'll download approximately 40GB of data (11GB CPAN, 28GB BackPAN)!

By default, mirropan will mirror both CPAN and BackPAN. As soon as you specify a `-source` parameter, both
CPAN and BackPAN mirrors will be disabled. You can add them again by specifying `-cpan` and `-backpan`.

You can disable only BackPAN or CPAN by passing the opposite flag. For example, to disable the BackPAN
mirror, pass the `-cpan` flag.

#### Command line options

| Option      | Example                                | Description
| ---------   | -------                                | -----------
| -h          | -h                                     | Display usage information
| -backpan    | -backpan                               | Add the default BackPAN source (only required if using -source)
| -cachedir   | -cachedir .gopancache                  | The GoPAN cache directory
| -cpan       | -cpan                                  | Add the default CPAN source (only required if using -source)
| -loglevel   | -loglevel=TRACE                        | Set log output level (ERROR, INFO, WARN, DEBUG, TRACE)
| -nocache    | -nocache                               | Ignore the cached index file
| -nomirror   | -nomirror                              | Don't mirror anything, just build the index
| -source     | -source DarkPAN=http://path.to/darkpan | Adds a *PAN source to mirror
| -update     | -update                                | Update the cached index file

## PANdex (pandex)

`pandex` builds a package index from a cached GoPAN index (created using mirropan).

It uses Module::Metadata and JSON::XS to extract metadata, and writes it to a packages index file.

Running pandex will overwrite any existing packages index file.

It also takes *ages* to run - approximately 48 hours on a 6-core i7 with 32GB memory and SSD!

You can set `-extdir` to change where pandex extracts archives to.
You should set this to a ramdisk to improve performance.

#### Command line options

| Option      | Example                                | Description
| ---------   | -------                                | -----------
| -h          | -h                                     | Display usage information
| -cachedir   | -cachedir .gopancache                  | The GoPAN cache directory
| -extdir     | -extdir /tmp/ramdisk                   | A temporary directory to extract archives to
| -loglevel   | -loglevel=TRACE                        | Set log output level (ERROR, INFO, WARN, DEBUG, TRACE)

## GetPAN (getpan)

Carton/cpanm compatible dependency manager

- Supports cpanfile syntax
