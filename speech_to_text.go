package speechtotext

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	speech "cloud.google.com/go/speech/apiv1"
	speechpb "google.golang.org/genproto/googleapis/cloud/speech/v1"

	"github.com/streadway/amqp"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
)

const (
	encodingAudioFormat string = "flac"
)

type Worker struct {
	SpeechClient              *speech.Client
	Ctx                       context.Context
	TranscoderMessageChannel  *amqp.Channel
	TranscoderMessageExchange string
}

type TranscodeInput struct {
	SourceFileUrl      string
	DestinationFileUrl string
	RespondTo          string
}

type RecognizeInput struct {
	SourceFileUrl      string
	DestinationFileUrl string
	LangCode           string
}

func NewWorker(transcoderChannel *amqp.Channel, transcoderExchange string) (*Worker, error) {
	ctx := context.Background()
	speechClient, err := speech.NewClient(ctx, option.WithServiceAccountFile(os.Getenv("GOOGLE_CREDENTIALS")))
	if err != nil {
		return &Worker{}, err
	}

	return &Worker{
		SpeechClient: speechClient,
		Ctx:          ctx,
		TranscoderMessageChannel:  transcoderChannel,
		TranscoderMessageExchange: transcoderExchange,
	}, nil
}

func (w *Worker) TranscodeAudioFile(in *TranscodeInput) error {
	data := map[string]string{
		"source_file_url":      in.SourceFileUrl,
		"destination_file_url": in.DestinationFileUrl,
		"respond_to":           in.RespondTo,
	}

	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	err = w.TranscoderMessageChannel.Publish(
		w.TranscoderMessageExchange, // exchange
		encodingAudioFormat,         // routing key
		false,                       // mandatory
		false,                       // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
	if err != nil {
		return err
	}

	return nil
}

func (w *Worker) Recognize(in *RecognizeInput) error {
	audioFileContent, err := w.downloadAudioFile(in.SourceFileUrl)
	if err != nil {
		return err
	}

	transcritpions, err := w.recognizeSpeech(audioFileContent, in.LangCode)
	if err != nil {
		return err
	}

	data, err := w.encodeData(transcritpions)
	if err != nil {
		return err
	}

	err = w.uploadResult(in.DestinationFileUrl, data)
	if err != nil {
		return err
	}

	return nil
}

func (w *Worker) downloadAudioFile(fileUrl string) ([]byte, error) {
	response, err := http.Get(fileUrl)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	return ioutil.ReadAll(response.Body)
}

func (w *Worker) recognizeSpeech(audioFileContent []byte, langCode string) ([]string, error) {
	resp, err := w.SpeechClient.Recognize(w.Ctx, &speechpb.RecognizeRequest{
		Config: &speechpb.RecognitionConfig{
			Encoding:        speechpb.RecognitionConfig_FLAC,
			SampleRateHertz: 16000,
			LanguageCode:    langCode,
		},
		Audio: &speechpb.RecognitionAudio{
			AudioSource: &speechpb.RecognitionAudio_Content{Content: audioFileContent},
		},
	})
	if err != nil {
		return nil, err
	}

	var transcriptions []string
	for _, result := range resp.Results {
		for _, alt := range result.Alternatives {
			transcriptions = append(transcriptions, alt.Transcript)
		}
	}

	return transcriptions, nil
}

func (w *Worker) encodeData(data []string) ([]byte, error) {
	body := map[string][]string{
		"result": data,
	}
	encBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	return encBody, nil
}

func (w *Worker) uploadResult(destinationUrl string, data []byte) error {
	req, err := http.NewRequest("PUT", destinationUrl, bytes.NewReader(data))
	if err != nil {
		return err
	}

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		errorBody, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}
		return errors.New(fmt.Sprintf("HTTP Error: %v, body: %s", res.StatusCode, errorBody))
	}

	return nil
}
