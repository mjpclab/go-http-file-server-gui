# Go HTTP File Server GUI

A GUI wrapper for go-http-file-server.

![Go HTTP File Server GUI](ghfs.webp)

## Prerequisites

The GUI uses [modernc.org/tk9.0](https://pkg.go.dev/modernc.org/tk9.0), which is
CGo-free and bundles the Tcl/Tk runtime — no C toolchain or system Tcl/Tk needed.
On Linux, an X11 display is required at runtime.

## Run from source

```sh
go run .
```

## Build

```sh
CGO_ENABLED=0 go build .
```

## Build and package

```sh
bash build/build-current.sh   # single binary for the current platform
bash build/build-all.sh       # cross-compile binaries for all platforms into output/
```
