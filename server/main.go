package main

import (
	"os"
	"fmt"
	"log"
	"net"

	// Imports the Google Cloud Speech API client package.
	"golang.org/x/net/context"

	"github.com/humaniq/speech_to_text/audio"

	speech "cloud.google.com/go/speech/apiv1"
	speechpb "google.golang.org/genproto/googleapis/cloud/speech/v1"

	"google.golang.org/grpc"
	"google.golang.org/api/option"
	"google.golang.org/grpc/reflection"
)

var (
	speech_client *speech.Client
)

type server struct {}

func (s *server) SpeechToText(ctx context.Context, in *audio.Request) (*audio.Response, error) {
	log.Println(fmt.Sprintf("Speech to text request received, language code: '%s'", in.LangCode))

	// Detects speech in the audio file.
	resp, err := speech_client.Recognize(ctx, &speechpb.RecognizeRequest{
		Config: &speechpb.RecognitionConfig{
			Encoding:        speechpb.RecognitionConfig_LINEAR16,
			SampleRateHertz: 16000,
			LanguageCode:    in.LangCode,
		},
		Audio: &speechpb.RecognitionAudio{
			AudioSource: &speechpb.RecognitionAudio_Content{Content: in.Audio},
		},
	})
	if err != nil {
		log.Println(err.Error())
	}

	var transcriptions []string
	for _, result := range resp.Results {
		for _, alt := range result.Alternatives {
			transcriptions = append(transcriptions, alt.Transcript)
		}
	}

	return &audio.Response{Transcriptions: transcriptions}, nil
}

func main() {
	ctx := context.Background()

	// Creates a client.
	var err error
	speech_client, err = speech.NewClient(ctx, option.WithServiceAccountFile(os.Getenv("GOOGLE_CREDENTIALS")))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Println(fmt.Sprintf("Server started at port: %s", "50051"))

	s := grpc.NewServer()
	audio.RegisterAudioServer(s, &server{})

	// Register reflection service on gRPC server.
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatal(err.Error())
	}
}
