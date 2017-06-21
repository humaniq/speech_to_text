# Speech to text service

## Install

Clone the repository:

```bash
$ git clone git@github.com:humaniq/speech_to_text.git
$ cd speech_to_text
```

Install dependencies:

```bash
$ brew install rabbitmq
$ brew install protobuf
$ go get -u github.com/golang/protobuf/protoc-gen-go
$ go get -u github.com/kardianos/govendor
$ govendor sync
```

Compile a `.proto` file:

```bash
$ protoc -I audio/ audio/audio.proto --go_out=plugins=grpc:audio
```

Run worker & server:

```bash
$ go run worker/main.go
$ go run server/main.go
```

Run the client (only for test/development):

```bash
$ go run test/client/main.go http://localhost:3000/audio.mp3
```
Argument: URL of audio file to recognize.

## Environment variables

* `APP_PORT` - The port on which the server is started. Default: '50052';
* `APP_ENV` – application environment.
* `GOOGLE_CREDENTIALS` - path to json file with credentials;
* `GOOGLE_STORAGE_BUCKET` - name of GCS bucket (default: `humaniq-speech`);
* `RABBITMQ_URL` - Default: `amqp://guest:guest@localhost:5672/`;
* `EXCHANGE_NAME` - Name of exchange in RabbitMQ (default: `humaniq-speech_to_text-exchange`);
* `TRANSCODER_EXCHANGE_NAME` - Name of exchange used to communicate with Transcoder service (default: `humaniq-transcoder-exchange`);
* `QUEUE_NAME` - Queue name for worker in RabbitMQ (default: `speech_to_text_worker`).
