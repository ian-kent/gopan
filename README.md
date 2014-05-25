GoPAN
=====

Perl dependency manager.

Builds a full dependency tree for cpanfile entries, downloading every CPAN archive to a local
cache, and installing them all in the right order with cpanm.

```
	gopan -cpanfile application.cpanfile -mirror http://your.mirror -mirror http://another.mirror
```

### command line options

|Option    |Example                     |Description
|--------- |-------                     |-----------
|-h        |-h                          |Display usage information
|-notest   |-notest AnyCache            |Disables module tests
|-cpus     |-cpus 4                     |Number of CPUs (or goroutines) to use
|-cpanfile |-cpanfile app.cpanfile      | The cpanfile to install from
|-mirror   |-mirror http://www.cpan.org | A mirror to use (can be specified multiple times)

### cpanfile extensions

#### Additional dependencies

If a module has a missing dependency which causes it to fail tests, you can fix it from the cpanfile
using a custom syntax:

    requires 'Broken::Module', '== 1.24'; # REQS: Missing::Dep-3.12; Another::Missing::Dep-1.82

### get_backpan_index.sh

Downloads the BackPAN index.

### get_perl_core.sh

Builds the list of core perl modules and pragmas.

These are added to perl_core.go, and are ignored when installing modules.
