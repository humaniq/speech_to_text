package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"

	// Imports the Google Cloud Speech API client package.
	"golang.org/x/net/context"

	"github.com/humaniq/speech_to_text/audio"

	speech "cloud.google.com/go/speech/apiv1"
	speechpb "google.golang.org/genproto/googleapis/cloud/speech/v1"

	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/humaniq/hmnqlog"
)

const (
	app_name    string = "speech_to_text"
	app_version string = "0.1.0"
)

var (
	speech_client *speech.Client
	hlog          hmnqlog.Logger
)

type server struct{}

func (s *server) SpeechToText(ctx context.Context, in *audio.Request) (*audio.Response, error) {
	hlog.Info(fmt.Sprintf("Speech to text request received, file URL: '%s', language code: '%s'", in.FileUrl, in.LangCode))

	audioFileContent, err := downloadAudioFile(in.FileUrl)
	if err != nil {
		hlog.Warn(err.Error())
		return nil, err
	}

	// Detects speech in the audio file.
	resp, err := speech_client.Recognize(ctx, &speechpb.RecognizeRequest{
		Config: &speechpb.RecognitionConfig{
			Encoding:        speechpb.RecognitionConfig_LINEAR16,
			SampleRateHertz: 16000,
			LanguageCode:    in.LangCode,
		},
		Audio: &speechpb.RecognitionAudio{
			AudioSource: &speechpb.RecognitionAudio_Content{Content: audioFileContent},
		},
	})
	if err != nil {
		hlog.Warn(err.Error())
		return nil, err
	}

	var transcriptions []string
	for _, result := range resp.Results {
		for _, alt := range result.Alternatives {
			transcriptions = append(transcriptions, alt.Transcript)
		}
	}

	return &audio.Response{Transcriptions: transcriptions}, nil
}

func downloadAudioFile(fileUrl string) ([]byte, error) {
	response, err := http.Get(fileUrl)
	if err != nil {
		return []byte{}, err
	}

	defer response.Body.Close()

	return ioutil.ReadAll(response.Body)
}

func main() {
	ctx := context.Background()

	var err error

	hlog, err = hmnqlog.NewZapLogger(hmnqlog.ZapOptions{
		AppName:     app_name,
		AppEnv:      os.Getenv("APP_ENV"),
		AppRevision: app_version,
	})
	if err != nil {
		log.Fatal(fmt.Sprintf("Failed to load logger, error %s", err.Error()))
	}

	// Creates a client.
	speech_client, err = speech.NewClient(ctx, option.WithServiceAccountFile(os.Getenv("GOOGLE_CREDENTIALS")))
	if err != nil {
		hlog.Fatal(fmt.Sprintf("Failed to create client: %v", err))
	}

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		hlog.Fatal(err.Error())
	}

	hlog.Info(fmt.Sprintf("Server started at port: %s", "50051"))

	s := grpc.NewServer()
	audio.RegisterAudioServer(s, &server{})

	// Register reflection service on gRPC server.
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		hlog.Fatal(err.Error())
	}
}
