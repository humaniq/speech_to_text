package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/satori/go.uuid"
	"github.com/streadway/amqp"

	"github.com/humaniq/hmnqlog"
	"github.com/humaniq/speech_to_text"
	"github.com/humaniq/speech_to_text/utils"
)

const (
	appName                       string = "speech_to_text_worker"
	appVersion                    string = "0.1.0"
	defaultExchangeName           string = "humaniq-speech_to_text-exchange"
	defaultTranscoderExchangeName string = "humaniq-transcoder-exchange"
	defaultQueueName              string = "speech_to_text_worker"
	defaultRabbitMQURL            string = "amqp://guest:guest@localhost:5672/"
)

var (
	hlog            hmnqlog.Logger
	requiredEnvVars = []string{"APP_ENV", "GOOGLE_CREDENTIALS"}
)

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

	conn, err := amqp.Dial(rabbitmqUrl())
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	err = utils.DeclareExchange(ch, exchangeName())
	failOnError(err, "Failed to declare an exchange")

	err = utils.DeclareExchange(ch, transcoderExchangeName())
	failOnError(err, "Failed to declare a transcoder exchange")

	q, err := utils.DeclareQueue(ch, queueName())
	failOnError(err, "Failed to declare a queue")

	err = utils.BindQueue(ch, q.Name, exchangeName())
	failOnError(err, "Failed to bind a queue")

	msgs, err := utils.ConsumeMessages(ch, q.Name)
	failOnError(err, "Failed to register a consumer")

	speechtotextWorker, err := speechtotext.NewWorker(ch, transcoderExchangeName())
	failOnError(err, "Failed to build speechtotext worker")

	forever := make(chan bool)

	go func() {
		for msg := range msgs {
			hlog.Info(fmt.Sprintf("Accept message: %s; routing: %s", msg.Body, msg.RoutingKey))

			err = handleMessage(speechtotextWorker, msg, ch)
			if err != nil {
				hlog.Warn(fmt.Sprintf("Error processing message: %v", err))
			}

			msg.Ack(false)

			hlog.Info("Done processing message")
		}
	}()

	hlog.Info("SpeechToText worker started")
	<-forever
}

func handleMessage(worker *speechtotext.Worker, msg amqp.Delivery, ch *amqp.Channel) error {
	var message map[string]string

	err := json.Unmarshal(msg.Body, &message)
	if err != nil {
		return err
	}

	callbackQueueName := fmt.Sprintf("%v", uuid.NewV4())
	callbackTo := fmt.Sprintf("%s|%s", exchangeName(), callbackQueueName)

	transcodeInput := &speechtotext.TranscodeInput{
		SourceFileUrl:      message["source_file_url"],
		DestinationFileUrl: message["upload_audio_file_url"],
		RespondTo:          callbackTo,
	}

	err = worker.TranscodeAudioFile(transcodeInput)
	if err != nil {
		return nil
	}

	go handleTranscoderCallback(worker, callbackQueueName, message, ch)
	return nil
}

func handleTranscoderCallback(worker *speechtotext.Worker, callbackQueueName string, originalMessage map[string]string, ch *amqp.Channel) {
	q, err := utils.DeclareQueue(ch, callbackQueueName)
	if err != nil {
		hlog.Warn(err.Error())
		return
	}
	err = utils.BindQueue(ch, q.Name, exchangeName())
	if err != nil {
		hlog.Warn(err.Error())
		return
	}

	msgs, err := utils.ConsumeMessages(ch, q.Name)
	if err != nil {
		hlog.Warn(err.Error())
		return
	}

	done := make(chan bool)
	go func() {
		for msg := range msgs {
			recognizeInput := &speechtotext.RecognizeInput{
				SourceFileUrl:      originalMessage["download_audio_file_url"],
				DestinationFileUrl: originalMessage["upload_result_file_url"],
				LangCode:           originalMessage["lang_code"],
			}
			err := worker.Recognize(recognizeInput)
			if err != nil {
				hlog.Warn(err.Error())
			}

			msg.Ack(false)
			done <- true
			return
		}
	}()
	<-done

	_, err = ch.QueueDelete(
		q.Name, //name
		false,  //ifUnused
		false,  //ifEmpty
		false,  //noWait
	)
	if err != nil {
		hlog.Warn(err.Error())
	}
	hlog.Info("Recognizing completed")
}

func rabbitmqUrl() string {
	return utils.FromEnvWithDefault("RABBITMQ_URL", defaultRabbitMQURL)
}
func exchangeName() string {
	return utils.FromEnvWithDefault("EXCHANGE_NAME", defaultExchangeName)
}
func transcoderExchangeName() string {
	return utils.FromEnvWithDefault("TRANSCODER_EXCHANGE_NAME", defaultTranscoderExchangeName)
}
func queueName() string {
	return utils.FromEnvWithDefault("QUEUE_NAME", defaultQueueName)
}

func failOnError(err error, msg string) {
	if err != nil {
		hlog.Fatal(fmt.Sprintf("%s: %s", msg, err))
	}
}
