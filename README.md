# tor-static

This project helps compile Tor into a static lib for use in other projects.

The dependencies are in this repository as submodules so this repository needs to be cloned with `--recursive`. The
submodules are:

* [OpenSSL](https://github.com/openssl/openssl/) - Checked out at tag `OpenSSL_1_0_2p`
* [Libevent](https://github.com/libevent/libevent) - Checked out at tag `release-2.1.8-stable`
* [zlib](https://github.com/madler/zlib) - Checked out at tag `v1.2.11`
* [XZ Utils](https://git.tukaani.org/?p=xz.git) - Checked out at tag `v5.2.4`
* [Tor](https://github.com/torproject/tor) - Checked out at tag `tor-0.3.5.2-alpha`

Many many bugs and quirks were hit while deriving these steps. Also many other repos, mailing lists, etc were leveraged
to get some of the pieces right. They are not listed here for brevity reasons.

**Note: Other versions of Tor may be available via tags (for current or previous versions) and branches (for future
versions)**

## Building

### Prerequisites

All platforms need Go installed and on the PATH.

#### Linux

Need:

* Normal build tools (e.g. `sudo apt-get install build-essential`)
* Libtool (e.g. `sudo apt-get install libtool`)
* autopoint (e.g. `sudo apt-get install autopoint`)

#### macOS

Need:

* Normal build tools (e.g. Xcode command line tools)
* Libtool (e.g. `brew install libtool`)
* Autoconf and Automake (e.g. `brew install automake`)
* autopoint (can be found in gettext, e.g. `brew install gettext`)
  * Note, by default this is assumed to be at `/usr/local/opt/gettext/bin`. Use `-autopoint-path` to change it.

#### Windows

Tor is not really designed to work well with MSVC so we use MinGW instead. In order to compile the dependencies,
Msys2 + MinGW should be installed.

Download and install the latest [MSYS2 64-bit](http://www.msys2.org/) that uses the `MinGW-w64` toolchains. Once
installed, open the "MSYS MinGW 64-bit" shell link that was created. Once in the shell, run:

    pacman -Syuu

Terminate and restart the shell if asked. Rerun this command as many times as needed until it reports that everything is
up to date. Then in the same mingw-64 shell, run:

    pacman -Sy --needed base-devel mingw-w64-i686-toolchain mingw-w64-x86_64-toolchain \
                        git subversion mercurial \
                        mingw-w64-i686-cmake mingw-w64-x86_64-cmake

This will install all the tools needed for building and will take a while. Once complete, MinGW is now setup to build 
the dependencies.

### Executing the build

In the cloned directory, run:

    go run build.go build-all

This will take a long time. Pieces can be built individually by changing the command from `build-all` to
`build-<folder>`. To clean, run either `clean-all` or `clean-<folder>`. To see the output of all the commands as they
are being run, add `-verbose` before the command.

## Using

Once the libs have been compiled, they can be used to link with your program. Due to recent refactorings within the Tor
source, the libraries are not listed here but instead listed when executing:

    go run build.go show-libs

This lists directories (relative, prefixed with `-L`) followed by lib names (file sans `lib` prefix and sans `.a`
extension, prefixed with `-l`) as might be used in `ld`.

The OS-specific system libs that have to be referenced (i.e. `-l<libname>`) are:

* Linux/macOS - `m`
* Windows (MinGW) - `ws2_32`, `crypt32`, and `gdi32`
