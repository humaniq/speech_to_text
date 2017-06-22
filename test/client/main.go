package main

import (
	"fmt"
	"log"
	"os"

	"github.com/humaniq/speech_to_text/audio"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var fileUrl string

func main() {
	args := os.Args
	if len(args) > 1 {
		fileUrl = args[1]
	} else {
		log.Fatal("You must pass a URL of audio file")
	}

	// Set up a connection to the server.
	conn, err := grpc.Dial("localhost:50052", grpc.WithInsecure())
	if err != nil {
		log.Fatal(err.Error())
	}
	defer conn.Close()

	c := audio.NewAudioClient(conn)
	r, err := c.SpeechToText(context.Background(), &audio.Request{LangCode: "en-US", FileUrl: fileUrl})
	if err != nil {
		log.Fatal(err.Error())
	}

	fmt.Println(r.FileUrl)
}
