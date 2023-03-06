# A CLI tool for managing Aerospike configuration files

asconfig currently supports...
- converting yaml files to aerospike config files.

# Usage
asconfig <path/to/config.yaml> [<path/to/aerospike.conf>] [flags]

## Usage examples

    Convert local file "aerospike.yaml" to aerospike config format for version 6.2.0.2 and
    write it to local file "aerospike.conf."
    ```
        asconfig --aerospike-version "6.2.0.2" aerospike.yaml aerospike.conf
    ```
    Short form flags and source file only conversions are also supported.
    In this case, -a is the server version and using only a source file means
    the result will be written as <path/to/config>.conf
    ```
        asconfig -a "6.2.0.2 aerospike.yaml
    ```

## Developer Docs

TODO