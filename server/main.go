package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/net/context"

	speech "cloud.google.com/go/speech/apiv1"
	speechpb "google.golang.org/genproto/googleapis/cloud/speech/v1"

	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/humaniq/grpc_proto/transcode"
	"github.com/humaniq/hmnqlog"
	"github.com/humaniq/speech_to_text/audio"

	"github.com/satori/go.uuid"
)

const (
	app_name                   string = "speech_to_text"
	app_version                string = "0.1.0"
	defaultGoogleStorageBucket string = "humaniq-speech"
	defaultTranscodeServiceUrl string = "localhost:50052"
)

var (
	speech_client     *speech.Client
	hlog              hmnqlog.Logger
	googleCredentials map[string]interface{}
)

type server struct{}

func (s *server) SpeechToText(ctx context.Context, in *audio.Request) (*audio.Response, error) {
	hlog.Info(fmt.Sprintf("Speech to text request received, file URL: '%s', language code: '%s'", in.FileUrl, in.LangCode))

	transcodedAudioFileUrl, err := transcodeAudioFile(in.FileUrl)
	if err != nil {
		hlog.Warn(fmt.Sprintf("Error transcoding audio file: %s", err))
		return nil, err
	}

	audioFileContent, err := downloadAudioFile(transcodedAudioFileUrl)
	if err != nil {
		hlog.Warn(fmt.Sprintf("Error downloading audio file: %s", err))
		return nil, err
	}

	// Detects speech in the audio file.
	resp, err := speech_client.Recognize(ctx, &speechpb.RecognizeRequest{
		Config: &speechpb.RecognitionConfig{
			Encoding:        speechpb.RecognitionConfig_FLAC,
			SampleRateHertz: 16000,
			LanguageCode:    in.LangCode,
		},
		Audio: &speechpb.RecognitionAudio{
			AudioSource: &speechpb.RecognitionAudio_Content{Content: audioFileContent},
		},
	})
	if err != nil {
		hlog.Warn(fmt.Sprintf("Error recognizing audio file: %s", err))
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

func transcodeAudioFile(fileUrl string) (string, error) {
	conn, err := grpc.Dial(transcodeServiceUrl(), grpc.WithInsecure())
	if err != nil {
		return "", err
	}
	defer conn.Close()

	filename := fmt.Sprintf("media/audio/%s", uuid.NewV4())

	destinationFileUrl, err := buildSignedURL(filename, "PUT")
	if err != nil {
		return "", err
	}
	transcodeRequest := &transcode.Request{SourceFileUrl: fileUrl, DestinationFileUrl: destinationFileUrl}
	transcodeClient := transcode.NewTranscodeClient(conn)

	_, err = transcodeClient.AudioToFlac(context.Background(), transcodeRequest)
	if err != nil {
		return "", err
	}

	resultFileUrl, err := buildSignedURL(filename, "GET")
	if err != nil {
		return "", err
	}
	return resultFileUrl, nil
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

func downloadAudioFile(fileUrl string) ([]byte, error) {
	response, err := http.Get(fileUrl)
	if err != nil {
		return []byte{}, err
	}

	defer response.Body.Close()

	return ioutil.ReadAll(response.Body)
}

func googleStorageBucket() string {
	bucket := os.Getenv("GOOGLE_STORAGE_BUCKET")
	if bucket == "" {
		bucket = defaultGoogleStorageBucket
	}
	return bucket
}

func transcodeServiceUrl() string {
	serviceUrl := os.Getenv("TRANSCODE_SERVICE_URL")
	if serviceUrl == "" {
		serviceUrl = defaultTranscodeServiceUrl
	}
	return serviceUrl
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

	err = loadGoogleCredentials(&googleCredentials)
	if err != nil {
		hlog.Fatal(fmt.Sprintf("Failed to load google credentials: %v", err))
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
