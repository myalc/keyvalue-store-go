# keyvalue-store-go

In memory key-value store REST-API written in Golang.

## Features 

Create, get single, delete all values.<br>
Writes all values to disk after an interval.<br>
When the application restarts checks the filesystem for a previous backup.<br>

### Create 
```sh
curl --location --request POST 'http://localhost:8080/api/v1/my/keys' \
--header 'Content-Type: application/json' \
--data-raw '{
    "key1": "value1"
}'
```

### Get Single 
```sh
curl --location --request GET 'http://localhost:8080/api/v1/my/keys/key1' \
--header 'Content-Type: application/json'
...
{
    "key1": "value1"
}
```

### Delete All 
```sh
curl --location --request DELETE 'http://localhost:8080/api/v1/my/keys' \
--header 'Content-Type: application/json'
```

## Install required Golang modules
```sh
go get github.com/google/uuid
go get -u github.com/swaggo/swag/cmd/swag
go get -u github.com/swaggo/http-swagger
go get -u github.com/alecthomas/template
```

## Run tests
```sh
cd goapp
go test
go test -v
go test -cover
```

## Run
```sh
cd goapp
go run .
```

## Docker
```sh
cd keyvalue-store-go
docker build -t myalc/keyvaluestore .
docker run -p 8080:8080 myalc/keyvaluestore
docker run -it run myalc/keyvaluestore /bin/bash
docker container prune
docker ps -a
```