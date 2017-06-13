package main

import (
	"os"
	"fmt"
	"log"
	"io/ioutil"

	"github.com/humaniq/speech_to_text/audio"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var path string

func main() {
	args := os.Args
	if len(args) > 1 {
		path = args[1]
	} else {
		log.Fatal("You must pass a path to audio file")
	}

	data, _ := ioutil.ReadFile(path)

	// Set up a connection to the server.
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatal(err.Error())
	}
	defer conn.Close()

	c := audio.NewAudioClient(conn)
	r, err := c.SpeechToText(context.Background(), &audio.Request{LangCode: "en-US", Audio: data})
	if err != nil {
		log.Fatal(err.Error())
	}

	fmt.Println(r.Transcriptions)
}
