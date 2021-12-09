## Build
FROM golang:1.16-buster AS build

COPY . /usr/src/myapp
WORKDIR /usr/src/myapp

RUN go mod download
RUN go build -o goapp


## Deploy
FROM gcr.io/distroless/base-debian10
WORKDIR /usr/src/myapp
COPY --from=build /usr/src/myapp .

EXPOSE 8080
CMD ["./goapp"]

LABEL Name=KeyValueStore Version=0.0.1
