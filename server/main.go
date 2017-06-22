package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/net/context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/humaniq/hmnqlog"
	"github.com/humaniq/speech_to_text/audio"
	"github.com/humaniq/speech_to_text/utils"

	"github.com/satori/go.uuid"
	"github.com/streadway/amqp"
)

const (
	appName                    string = "speech_to_text"
	appVersion                 string = "0.1.0"
	defaultAppPort             string = "50052"
	defaultExchangeName        string = "humaniq-speech_to_text-exchange"
	defaultQueueName           string = "speech_to_text_worker"
	defaultRabbitMQURL         string = "amqp://guest:guest@localhost:5672/"
	defaultGoogleStorageBucket string = "humaniq-speech"
)

var (
	hlog                hmnqlog.Logger
	googleCredentials   map[string]interface{}
	messageQueueChannel *amqp.Channel
	requiredEnvVars     = []string{"GOOGLE_CREDENTIALS", "APP_ENV"}
)

type server struct{}

func (s *server) SpeechToText(ctx context.Context, in *audio.Request) (*audio.Response, error) {
	hlog.Info(fmt.Sprintf("Speech to text request received, file URL: '%s', language code: '%s'", in.FileUrl, in.LangCode))

	audioFileName := fmt.Sprintf("media/audio/%s", uuid.NewV4())
	uploadAudioFileUrl, err := buildSignedURL(audioFileName, "PUT")
	if err != nil {
		return nil, err
	}
	downloadAudioFileUrl, err := buildSignedURL(audioFileName, "GET")
	if err != nil {
		return nil, err
	}

	resultFileName := fmt.Sprintf("data/text/%s", uuid.NewV4())
	uploadResultFileUrl, err := buildSignedURL(resultFileName, "PUT")
	if err != nil {
		return nil, err
	}
	downloadResultFileUrl, err := buildSignedURL(resultFileName, "GET")
	if err != nil {
		return nil, err
	}

	err = createSpeechToTextTask(in.FileUrl, in.LangCode, uploadAudioFileUrl, downloadAudioFileUrl, uploadResultFileUrl)
	if err != nil {
		return nil, err
	}

	return &audio.Response{FileUrl: downloadResultFileUrl}, nil
}

func buildSignedURL(filename, method string) (string, error) {
	url, err := storage.SignedURL(googleStorageBucket(), filename, &storage.SignedURLOptions{
		GoogleAccessID: googleCredentials["client_email"].(string),
		PrivateKey:     []byte(googleCredentials["private_key"].(string)),
		Method:         method,
		Expires:        time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		return "", err
	}

	return url, nil
}

func createSpeechToTextTask(sourceFileUrl, langCode, uploadAudioFileUrl, downloadAudioFileUrl, uploadResultFileUrl string) error {
	message := map[string]string{
		"source_file_url":         sourceFileUrl,
		"lang_code":               langCode,
		"upload_audio_file_url":   uploadAudioFileUrl,
		"download_audio_file_url": downloadAudioFileUrl,
		"upload_result_file_url":  uploadResultFileUrl,
	}

	body, err := json.Marshal(message)
	if err != nil {
		return err
	}

	err = messageQueueChannel.Publish(
		exchangeName(), // exchange
		queueName(),    // routing key
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})

	if err != nil {
		return err
	}
	return nil
}

func main() {
	err := utils.CheckRequiredEnvVars(requiredEnvVars)
	if err != nil {
		log.Fatal(err)
	}

	hlog, err = hmnqlog.NewZapLogger(hmnqlog.ZapOptions{
		AppName:     appName,
		AppEnv:      os.Getenv("APP_ENV"),
		AppRevision: appVersion,
	})
	if err != nil {
		log.Fatal(fmt.Sprintf("Failed to load logger, error %s", err.Error()))
	}

	err = loadGoogleCredentials(&googleCredentials)
	if err != nil {
		hlog.Fatal(fmt.Sprintf("Failed to load google credentials: %v", err))
	}

	var messageQueueConnection *amqp.Connection
	messageQueueConnection, messageQueueChannel = setupMessageQueueConnection()
	defer messageQueueConnection.Close()
	defer messageQueueChannel.Close()

	lis, err := net.Listen("tcp", ":"+appPort())
	if err != nil {
		hlog.Fatal(err.Error())
	}

	hlog.Info(fmt.Sprintf("Server started at port: %s", appPort()))

	s := grpc.NewServer()
	audio.RegisterAudioServer(s, &server{})

	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		hlog.Fatal(err.Error())
	}
}

func loadGoogleCredentials(credentials *map[string]interface{}) error {
	jsonFile, err := ioutil.ReadFile(os.Getenv("GOOGLE_CREDENTIALS"))
	if err != nil {
		return err
	}

	err = json.Unmarshal(jsonFile, credentials)
	if err != nil {
		return err
	}

	return nil
}

func setupMessageQueueConnection() (*amqp.Connection, *amqp.Channel) {
	conn, err := amqp.Dial(rabbitmqUrl())
	failOnError(err, "Failed to connect to RabbitMQ")

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")

	err = utils.DeclareExchange(ch, exchangeName())
	failOnError(err, "Failed to declare an exchange")
	return conn, ch
}

func appPort() string {
	return utils.FromEnvWithDefault("APP_PORT", defaultAppPort)
}
func rabbitmqUrl() string {
	return utils.FromEnvWithDefault("RABBITMQ_URL", defaultRabbitMQURL)
}
func exchangeName() string {
	return utils.FromEnvWithDefault("EXCHANGE_NAME", defaultExchangeName)
}
func queueName() string {
	return utils.FromEnvWithDefault("QUEUE_NAME", defaultQueueName)
}
func googleStorageBucket() string {
	return utils.FromEnvWithDefault("GOOGLE_STORAGE_BUCKET", defaultGoogleStorageBucket)
}

func failOnError(err error, msg string) {
	if err != nil {
		hlog.Fatal(fmt.Sprintf("%s: %s", msg, err))
	}
}
