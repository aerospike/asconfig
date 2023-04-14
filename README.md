# Asconfig

A CLI tool for managing Aerospike configuration files.

# Overview

Asconfig allows you to manage and create Aerospike configuration using a versioned schema directory. This configuration is shared with the Aerospike cluster Custom Resource.
To get started you can copy an example below or load the schema into your IDE.
Run `asconfig convert -a <aerospike-version> <path/to/config.yaml>` to convert your yaml configuration to an [Aerospike configuration file](https://docs.aerospike.com/server/operations/configure).
The converted file can be used to configure the Aerospike database.

Run `asconfig --help` and see the sections below for more details.

# Usage

asconfig [command]

## Supported commands

| Name | Description |
| ---- | ----------- |
| convert | convert yaml to aerospike config format |

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
```
make rpm deb tar
```
The packages will be available in the pkg/ directory.
