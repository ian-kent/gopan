SmartPAN
========

SmartPAN is a CPAN, BackPAN or DarkPAN hosting tool.

It provides a CPAN-compatible index which is supported by
both Carton and cpanm.

It also provides a 'SmartPAN' interface which is supported
by GetPAN for more effective dependency resolution.

## Features

- Web UI to browse and import modules
- Import from 
  - cpanfile or .tar.gz releases
  - other CPAN mirrors
  - URL or local disk
  - Upload with curl or a web browser
- Search by
  - module name
  - release archive name
  - module packages
- SmartPAN index
  - exposes latest modules across all indexes
  - can also be used as a CPAN mirror
- Command line import
  - To local or remote SmartPAN
  - From local file or URL

## Getting started

Run `smartpan` and open `http://your.host:7050` in a browser.

- Import some modules
- Pass it to GetPAN, Carton, cpanm as a mirror:

  - `getpan -smart http://your.host:7050`
  - `getpan -cpan http://your.host:7050/YourIndex`
  - `PERL_CARTON_MIRROR=http://your.host:7050/YourIndex carton install`
  - `cpanm --mirror http://your.host:7050/YourIndex Some::Module`

You can also use the SmartPAN index URL:

  - `getpan -cpan http://your.host:7050/SmartPAN`
  - `PERL_CARTON_MIRROR=http://your.host:7050/SmartPAN carton install`
  - `cpanm --mirror http://your.host:7050/SmartPAN Some::Module`

## Command line options

| Option            | Example                          | Description
| ---------         | -------                          | -----------
| -h                | -h                               | Display usage information
| -bind             | -bind 0.0.0.0:7050               | The interface and port to bind to
| -cachedir         | -cachedir .gopancache            | Set the cache directory
| -cpan             | -cpan "some_index.gz"            | Name of CPAN index to support readthrough
| -index            | -index index                     | The name of the index file
| -loglevel         | -loglevel=TRACE                  | Set log output level (ERROR, INFO, WARN, DEBUG, TRACE)
| -remote           | -remote=http://somewhere:7050    | Set the remote SmartPAN
| -s-index          | -s-index="index_name"            | Secondary indexes to load

### Importing from the command line

You can import modules from the command line using SmartPAN.

    smartpan import /path/to/File-0.01.tar.gz AUTHORID IndexName
    smartpan import http://somewhere/File-0.01.tar.gz AUTHORID IndexName

You can also import into remote SmartPAN hosts.

    smartpan -remote http://smartpan:7050 /path/to/File-0.01.tar.gz AUTHORID IndexName
    smartpan -remote http://smartpan:7050 import http://somewhere/File-0.01.tar.gz AUTHORID IndexName

## HTTP API

You can query SmartPAN using HTTP to locate modules.

### List all versions of a module

    curl -X GET http://path.to/SmartPAN/where/Module::Name

### List all matching versions of a module

    curl -X GET http://path.to/SmartPAN/where/Module::Name/1.92
    curl -X GET http://path.to/SmartPAN/where/Module::Name/==3.99
    curl -X GET http://path.to/SmartPAN/where/Module::Name/>=2.00

## Licence

Copyright ©‎ 2014, Ian Kent (http://www.iankent.eu).

Released under MIT license, see [LICENSE](LICENSE.md) for details.
