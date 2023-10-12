# Program VM simulator

## Introduction

The VM simulator provides a tool for testing and interacting with HyperSDK Wasm
`Programs`.

## Build

```sh
go build
./simulator -h
```

## Example

There is a full example of the `token` program in both `YAML` and `JSON` format
located in the `testdata/` directory.

```sh
./simulator program run ./testdata/token.json 
```

## Import Modules

Currently the simulator supports the `program` and `pstate` modules found in the
examples/imports directory.




