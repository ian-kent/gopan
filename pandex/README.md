PANdex
======

PANdex is a GoPAN cache indexer.

It uses Parse::LocalDistribution to extract package metadata from
CPAN .tar.gz releases found in the gopancache directory.

## Features

- Generates or updates a PANdex index file
- Indexes a MirroPAN mirror

## Getting started

Run PANdex from the command line to generate a packages index:

    pandex

The packages index will be output to `.gopancache/packages` to avoid
overwriting an existing PANdex `.gopancache/index` file.

Any existing `.gopancache/packages` file will be overwritten.


It uses Perl, Parse::LocalDistribution and JSON::XS to extract metadata.

**WARNING**

PANdex will extract each .tar.gz file sequentially.

You should use a ramdisk and specify the `-extdir` parameter to decompress in-memory.

If you are indexing a full BackPAN mirror, this is horrifically slow. 
It takes approximately 48 hours on a 6-core i7 with 32GB memory and SSD.

## Command line options

| Option      | Example                                | Description
| ---------   | -------                                | -----------
| -h          | -h                                     | Display usage information
| -cachedir   | -cachedir .gopancache                  | The GoPAN cache directory
| -extdir     | -extdir /tmp/ramdisk                   | A temporary directory to extract archives to
| -loglevel   | -loglevel=TRACE                        | Set log output level (ERROR, INFO, WARN, DEBUG, TRACE)


## Licence

Copyright ©‎ 2014, Ian Kent (http://www.iankent.eu).

Released under MIT license, see [LICENSE](LICENSE.md) for details.
