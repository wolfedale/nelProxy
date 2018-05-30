# nel-proxy
Simple API for Jenkins->Ansible tasks

Many times there is a case where we have few production systems, we manage them by ansible but somehow we still want to have one single place from which we can execute tasks on them.
We can do this from Jenkins (or anything else from which we can call API and send JSON struct to it).

## How it works

nel-proxy has been written in Go, which means we do have only one binary file. This binary file have two options, server mode and worker mode.
Server mode needs to ne run somewhere between Jenkins and our production systems, so Jenkins can call it and each production system as well.
Over this server (api) we will send JSON structs with ansible tasks.

Example of how to start it:
```
  $ go build main.go
  $ ./nelProxy --server=10.0.0.24 --ssl=false --logs=./NelProxy.log --port=8080
```

Where server is of course bind IP address, ssl stands for SSL, we can enable it or disable, we also need to say where we want to keep logs and on which port we would like to start it.

Example of how to run worker mode:
```
  $ go build main.go
  $ ./nelProxy --server=10.0.0.24 --ssl=false --logs=./NelProxy.log --port=8080 --worker=true --inventory={{production inventory file}}
```

Same like server mode, except we need to specify inventory file in ansible and also say that this is a worker, not a server.

Example of how to call API:
```
  $ curl -H "Content-Type: application/json" -d @test.json http://10.0.0.24:8080/task
```

How to delete task with ID 2:
```
  $ curl -X "DELETE" http://10.0.0.24:8080/task/2
```

How to get full list of all tasks:
```
  $ curl http://10.0.0.24:8080/task | jq
```

## How to install it

Since it's written in Go we do need to compile it
```
  $ go build main.go
```

And simply copy binary file to the production system and somewhere where we want to run API

## Contribution

Please do!

## Author

- Pawel Grzesik <pawel.grzesik@gmail.com>
