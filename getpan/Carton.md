Differences to Carton
=====================

### Carton updates Perl core modules

If your cpanfile lists a module which has a dependency on a Perl
core module, the core module will be updated if later versions are
available from CPAN.

For example, Module::Build will be updated when using Perl 5.18.2.

This is unexpected behaviour and could potentially destabilise the
official Perl release.

GetPAN intentionally excludes Perl core modules (see [../DEVELOPING.md](../DEVELOPING.md)).

To force Perl core modules to update, add them to your cpanfile.

### Carton delegates dependency resolution

Installing a cpanfile with Carton is essentially passing the required
module to cpanm, which resolves dependencies during installation.

This limits the potential to optimise module installation.

GetPAN takes full control of dependency resolution, constructing a
full dependency tree before beginning module installation.

By doing this, GetPAN can simultaneously install multiple modules
at a time, ensuring dependencies are already installed before passing
the module to cpanm.

This can reduce dependency installation time by a factor of 4.
