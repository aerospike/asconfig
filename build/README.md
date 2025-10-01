### Build Scripts

### build_package_int

** FOR AEROSPIKE USAGE ONLY **

This script basically builds the tools package  based on the *CLIENTREPO* environment variable.

This also temporarily creates symlinks to the *client* folder for building successfully.

```
ln -s $CLIENTREPO/client client
ln -s $CLIENTREPO/shared/include client/include
ln -s $CLIENTREPO/client/c_clients/cl_c/lib client/lib
```


### build_package

This is a script builds client package based on the SDK downloaded from the website

It is important to download it in base directory of *aerospike-tools* repo

``` 
$ git clone git@github.com:aerospike/aerospike-tools.git
$ cd aerospike-tools
$ wget http://www.aerospike.com/client_downloads/c/citrusleaf_client_c_2.1.11.tgz
$ tar -zxvf citrusleaf_client_c_2.1.11.tgz
$ mv citrusleaf_client_c_2.1.11 client
```


### build

This just does a *make clean;make* , Assuming you have client SDK downloaded.


### package

Package the tools assuming build has been run.
