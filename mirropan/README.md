MirroPAN
========

MirroPAN is a CPAN, BackPAN and DarkPAN mirroring tool.

It brute-force mirrors a *PAN mirror through web scraping,
allowing it to work behind a HTTP proxy.

## Features

- Brute force HTTP mirroring of CPAN/BackPAN/DarkPAN sources
- Builds a [PANdex](../pandex/README.md) compatible index

## Getting started

**WARNING** By default, it'll download approximately 40GB of data 
(11GB CPAN, 28GB BackPAN)!

Run MirroPAN from the command line:

    mirropan

By default, mirropan will mirror both CPAN and BackPAN. 

As soon as you specify a `-source` parameter, both CPAN and BackPAN
mirrors will be disabled. You can add them again by specifying `-cpan` 
and `-backpan`.

You can disable only BackPAN or CPAN by passing the opposite flag. 
For example, to disable the BackPAN mirror, pass the `-cpan` flag.

## Command line options

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

## Licence

Copyright ©‎ 2014, Ian Kent (http://www.iankent.eu).

Released under MIT license, see [LICENSE](LICENSE.md) for details.
