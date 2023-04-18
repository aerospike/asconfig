# Asconfig

A CLI tool for managing Aerospike configuration files.

# Overview

Asconfig allows you to manage and create Aerospike configuration using a versioned schema directory. This configuration is shared with the Aerospike cluster Custom Resource.
To get started you can copy an example below or load the schema into your IDE.
Run `asconfig convert -a <aerospike-version> <path/to/config.yaml>` to convert your yaml configuration to an [Aerospike configuration file](https://docs.aerospike.com/server/operations/configure).
The converted file can be used to configure the Aerospike database.

Run `asconfig --help` and see the sections below for more details.

# Usage

`asconfig COMMAND [flags] [arguments]`

## Supported Commands

| Command | Description |
| ------- | ----------- |
| completion [flags] <shell> | Generate the autocompletion script for the specified shell |
| convert [flags] <path/to/config.yaml> | Convert yaml to aerospike config format |
| help [command] | Help about any command |

## Usage examples

Convert local file "aerospike.yaml" to aerospike config format for Aerospike server version 6.2.0 and
write it to local file "aerospike.conf."

```shell
    asconfig convert --aerospike-version "6.2.0" aerospike.yaml --output aerospike.conf
```

Short form flags and source file only conversions are also supported.
In this case, -a is the server version and using only a source file means
the result will be written to stdout.

```shell
    asconfig convert -a "6.2.0" aerospike.yaml
```

## Schema validation

Installing the [Red Hat YAML vscode extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml) is recommended. The extension allows using the Aerospike configuration json schema files for code suggestions in vscode when creating your own yaml configuration.

The json schema files used by asconfig and in this example are stored in the [asconfig schema directory](https://github.com/aerospike/asconfig/tree/main/schema/json). In order to use them for writing your own yaml config, clone the [asconfig github repository](https://github.com/aerospike/asconfig) and follow the example below.

### Example

You can load schema files into most IDE's to get code suggestions. The following steps walk through this process in vscode.

- Install the [Red Hat YAML vscode extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml).

- In vscode, go to preferences, then settings. Search for "YAML schema" and click "edit in settings.json".

- Add a yaml.schemas mapping like the one below to your settings.json. Replace "/absolute/path/to/asconfig/repo" with the path to your local clone of the asconfig repo.

    ```json
        "yaml.schemas": {
            "/absolute/path/to/asconfig/repo/schema/json/6.2.0.json": ["/*aerospike.yaml"]
        }
    ```

    This will associate all files ending in "aerospike.yaml" with the 6.2.0 Aerospike yaml schema.

Now you can use the code suggestions from the 6.2.0 Aerospike yaml schema to write your yaml configuration.

## Configuration Examples

Here is an example yaml config and the command to convert it to an [Aerospike configuration file](https://docs.aerospike.com/server/operations/configure) for database version 6.2.0.x.

### example.yaml

```yaml
service:
  feature-key-file: /etc/aerospike/features.conf

logging:
- name: console
  any: info
network:
  service:
    port: 3000
  fabric:
    port: 3001
  heartbeat:
    mode: mesh
    port: 3002
    addresses: 
      - local

xdr:
  dcs: 
    - name: elastic
      connector: true
      node-address-ports:
        -  0.0.0.0 8080
      namespaces:
        - name: test

namespaces:
  - name: test
    memory-size: 3000000000
    replication-factor: 2
    storage-engine:
      type: device
      files:
        - /opt/aerospike/data/test.dat
      filesize: 2000000000
      data-in-memory: true

```

### Convert

```shell
asconfig convert -a 6.2.0 example.yaml -o example.conf
```

For More examples see the aerospikeConfig property from the [Aerospike Kubernetes Operator examples](https://github.com/aerospike/aerospike-kubernetes-operator/tree/master/config/samples).

## Build

Build asconfig using the included top level makefile.

```shell
make
```

The resulting binary is available at bin/asconfig

Building rpm, deb, and tar packages is also done using the makefile.
You will have to install fpm and rpmbuild to build all of these.

```shell
make rpm deb tar
```

The packages will be available in the pkg/ directory.
