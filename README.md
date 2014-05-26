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
	gopan -cpanfile application.cpanfile -mirror http://your.mirror -mirror http://another.mirror
```

### command line options

| Option     | Example                     | Description
| ---------  | -------                     | -----------
| -h         | -h                          | Display usage information
| -notest    | -notest AnyCache            | Disables module tests
| -cpus      | -cpus 4                     | Number of CPUs (or goroutines) to use
| -cpanfile  | -cpanfile app.cpanfile      | The cpanfile to install from
| -mirror    | -mirror http://www.cpan.org | A mirror to use (can be specified multiple times)
| -noinstall | -noinstall                  | If provided, install phase will be skipped
| -loglevel  | -loglevel=TRACE             | Set log output level (ERROR, INFO, WARN, DEBUG, TRACE)

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

### get_perl_core.sh

Only required if building GoPAN from source.

Builds the list of core perl modules and pragmas.

These are added to perl_core.go, and are ignored when installing modules.

## To-do

- Better cpanfile syntax support
  - author path (A/AB/ABC/Some-Module-1.01.tar.gz)
  - full URL (http://path.to/Some-Module-1.01.tar.gz)
- BackPAN version lookup support
  - multiple BackPAN indexes/URLs
  - index BackPAN versions so "Some::Module-1.01" is found for "Some::Module >= 1.00"
