# A CLI tool for managing Aerospike configuration files

Asconfig currently supports converting from yaml to asconfig.

# Usage

asconfig [command]

## Supported commands

| Name | Description |
| ---- | ----------- |
| convert | convert yaml to aerospike config format |

## Usage examples

    Convert local file "aerospike.yaml" to aerospike config format for Aerospike server version 6.2.0 and
    write it to local file "aerospike.conf."
    ```
        asconfig convert --aerospike-version "6.2.0" aerospike.yaml --output aerospike.conf
    ```
    Short form flags and source file only conversions are also supported.
    In this case, -a is the server version and using only a source file means
    the result will be written to stdout.
    ```
        asconfig convert -a "6.2.0" aerospike.yaml
    ```
