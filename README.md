GoPAN
=====

A multithreaded Perl dependency manager.

- Builds a full dependency tree from a cpanfile
- Downloads CPAN archives to local cache
- Resolves dependency installation order
- Supports multiple CPAN mirrors
- Supports BackPAN for old module versions
- Per-module notest and dependency fixes

```
    ./get_backpan_index.sh
	gopan -cpanfile application.cpanfile -cpan http://your.mirror -cpan http://another.mirror
```

### command line options

| Option            | Example                          | Description
| ---------         | -------                          | -----------
| -h                | -h                               | Display usage information
| -backpan          | -backpan http://backpan.perl.org | A BackPAN mirror to use (can be specified multiple times)
| -cpan             | -cpan http://www.cpan.org        | A CPAN mirror to use (can be specified multiple times)
| -cpanfile         | -cpanfile app.cpanfile           | The cpanfile to install from
| -cpus             | -cpus 4                          | Number of CPUs to use
| -loglayout        | -loglayout="[%d] %m"             | A github.com/ian-kent/go-log compatible pattern layout
| -loglevel         | -loglevel=TRACE                  | Set log output level (ERROR, INFO, WARN, DEBUG, TRACE)
| -nevertest        | -nevertest                       | Disables all installation tests
| -noinstall        | -noinstall                       | If provided, install phase will be skipped
| -notest           | -notest AnyCache                 | Disables module tests

### cpanfile

#### Basic syntax

Supports only basic cpanfile syntax:

	# Latest version
    requires 'Module::Name';

    # Minimum version
    requires 'Module::Name', '1.02';
    requires 'Module::Name', '>= 1.02';

    # Exact version
    requires 'Module::Name', '== 1.02';

    # Maximum version
    requires 'Module::Name', '<= 1.02';

#### Additional dependencies

If a module has a missing dependency which causes it to fail tests, you can fix it from the cpanfile
using a custom syntax:

    requires 'Broken::Module', '== 1.24'; # REQS: Missing::Dep-3.12; Another::Missing::Dep-1.82

### get_backpan_index.sh

Downloads the BackPAN index.

### Why?

It's faster. And it gets dependencies right.

And its written in Go - so no installation!

#### With Carton

    $ rm ./local -rf
    $ PERL_CARTON_MIRROR=http://*****:5888 time carton
    148 distributions installed
    Installing modules failed

    real    3m5.707s
    user    1m52.291s
    sys	    0m25.856s

#### With GoPAN

    $ rm ./local -rf
    $ time gopan -cpan http://****:5888 -nevertest
    [INFO] Successfully installed 258 modules

    real    0m46.274s
    user    3m35.914s
    sys     0m31.583s

## Contributing

### get_perl_core.sh

Only required if building GoPAN from source.

Builds the list of core perl modules and pragmas.

These are added to perl_core.go, and are ignored when installing modules.

Current perl_core.go is against perl 5.18.2.

### To-do

- Better cpanfile syntax support
  - author path (A/AB/ABC/Some-Module-1.01.tar.gz)
  - full URL (http://path.to/Some-Module-1.01.tar.gz)
- BackPAN version lookup support
  - multiple BackPAN indexes/URLs
  - index BackPAN versions so "Some::Module-1.01" is found for "Some::Module >= 1.00"
- gopan exec?

### Pull requests

Before submitting a pull request:

  * Format your code: ```go fmt ./...```

### Licence

Copyright ©‎ 2014, Ian Kent (http://www.iankent.eu).

Released under MIT license, see [LICENSE](LICENSE.md) for details.
