# go-stdlib

[![CI](https://github.com/opentracing-contrib/go-stdlib/actions/workflows/ci.yml/badge.svg)](https://github.com/opentracing-contrib/go-stdlib/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/opentracing-contrib/go-stdlib)](https://goreportcard.com/report/github.com/opentracing-contrib/go-stdlib)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/opentracing-contrib/go-stdlib)
[![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/opentracing-contrib/ggo-stdlibc?logo=github&sort=semver)](https://github.com/opentracing-contrib/go-stdlib/releases/latest)

This repository contains OpenTracing instrumentation for packages in
the Go standard library.

For documentation on the packages,
[check godoc](https://godoc.org/github.com/opentracing-contrib/go-stdlib/).

**The APIs in the various packages are experimental and may change in
the future. You should vendor them to avoid spurious breakage.**

## Packages

Instrumentation is provided for the following packages, with the
following caveats:

- **net/http**: Client and server instrumentation. *Only supported
  with Go 1.7 and later.*

## License

By contributing to this repository, you agree that your contributions will be licensed under [Apache 2.0 License](./LICENSE).
