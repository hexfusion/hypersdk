# Program VM simulator

## Introduction

The VM simulator provides a tool for testing and interacting with HyperSDK Wasm
`Programs`.

## Build

```sh
go build
./simulator -h
```

## Testing a HyperSDK Programs

To test a HyperSDK Program you will need to create a `run` file. We currently support `run` files in both `JSON` and `YAML`.

### Deploy a HyperSDK Program

Deploying a HyperSDK Program to the VM Simulator can be as simple as.

# create a key
```
./simulator key create my_key
database: /home/dev/.simulator/db
created new private key: my_key
```

# deploy
```
./simulator program deploy ./my_program.wasm --key my_key
```


In this example we will create a new key `my_key` and deploy a new program using
a run file.

```yaml
# new_program.yaml
# example of creating a new key and deploying a new program using a run file
name: new program
description: Deploy a new program
caller_key: my_key
steps:
  - name: create_key
    description: create my key
    key_name: my_key
  - name: deploy
    description: deploy my new program
    program_path: new_program.wasm
    max_fee: 10000
```

Next we will run the simulation

```sh
$./simulator program run ./new_program.yaml

database: /home/dev/.simulator/db
simulating: new program

step: 0
description: create my key
key creation successful: my_key

step: 1
description: deploy a new program
deploy transaction successful: 2Ej3Qp6aUZ7yBnqZxBmvvvekUiriCn4ftcqY8VKGwMu5CmZiz
```

Congratulations you have just deployed your first HyperSDK program! Lets make
sure to keep track of the transaction ID
`2Ej3Qp6aUZ7yBnqZxBmvvvekUiriCn4ftcqY8VKGwMu5CmZiz`.

### Interact with a HyperSDK Program

Now that the program is on chain lets interact with it.

```yaml
# play_program.yaml
name: play
description: Playing with new program
caller_key: my_key
steps:
  - name : call
    description: add two numbers
    function: add
    program_id: 2Ej3Qp6aUZ7yBnqZxBmvvvekUiriCn4ftcqY8VKGwMu5CmZiz
    max_fee: 100000
    params:
      - type: uint64
        value: 100
      - type: uint64
        value: 100
    require:
      result:
        operator: "=="
        operand: 200
```

Run simulation

```sh
$./simulator program run ./testdata/token.json

database: /home/dev/.simulator/db
simulating: new program

step: 0
description: add two numbers
function: add
params: [{uint64 100} {uint64 100}]
max fee: 100000
fee balance: 98225
response: 200
call transaction successful: 3QoxsNhkUN21iwR4LeMxfSrpBD4c7vKs9aJXbEdi6FeHWNJVu
```

## Deploy and Interact with HyperSDK Program

The above examples show how to deploy and interact with a `HyperSDK` program in
seperate run files but we can also perform all of this in a single run. See the
example in the `token.yaml` file of using the `inherit` keyword.

## Example

There is a full example of the `token` program in both `YAML` and `JSON` format
located in the `testdata/` directory.

```sh
./simulator program run ./testdata/token.json 
```

## Import Modules

Currently the simulator supports the `program` and `pstate` modules found in the
examples/imports directory.
