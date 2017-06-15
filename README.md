# Speech to text service

## Install

Clone the repository:

```bash
$ git clone git@github.com:humaniq/speech_to_text.git
$ cd speech_to_text
```

Install dependencies:

```bash
$ brew install protobuf
$ go get -u github.com/golang/protobuf/protoc-gen-go
$ go get -u github.com/kardianos/govendor
$ govendor sync
```

Compile a `.proto` file:

```bash
$ protoc -I audio/ audio/audio.proto --go_out=plugins=grpc:audio
```

Run the server:

```bash
$ go run server/main.go
```

Run the client (Only for test):

```bash
$ go run client/main.go audio.raw
```

## Environment variables

* `GOOGLE_CREDENTIALS` - path to json file with credentials;
* `GOOGLE_STORAGE_BUCKET` - name of GCS bucket (default: `humaniq-speech`);
* `TRANSCODE_SERVICE_URL` - URL of Transcode service (default: `localhost:50052`);
* `APP_ENV` – application environment.
