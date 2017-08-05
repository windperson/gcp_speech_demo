// Copyright 2016 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Command livecaption pipes the stdin audio data to
// Google Speech API and outputs the transcript.
//
// As an example, gst-launch can be used to capture the mic input on Ubuntu Linux:
// 1. Install gstreamer-tools: apt-get install gstreamer-tools
// 2. Run go build, then run following command line:
// gst-launch-1.0 -v alsasrc ! audioconvert ! audioresample ! audio/x-raw,channels=1,rate=16000 ! filesink location=/dev/stdout | ./gcp_speech_demo
package main

import (
	"io"
	"log"
	"os"

	"time"

	"cloud.google.com/go/speech/apiv1"
	"golang.org/x/net/context"
	speechpb "google.golang.org/genproto/googleapis/cloud/speech/v1"
)

func main() {
	bgCtx := context.Background()
	ctx, _ := context.WithDeadline(bgCtx, time.Now().Add(205*time.Second))

	// [START speech_streaming_mic_recognize]
	client, err := speech.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	stream, err := client.StreamingRecognize(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Send the initial configuration message.
	os.Stderr.WriteString("sending init StreamingConfig...\n")
	if err := stream.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: &speechpb.StreamingRecognitionConfig{
				Config: &speechpb.RecognitionConfig{
					Encoding:        speechpb.RecognitionConfig_LINEAR16,
					SampleRateHertz: 16000,
					LanguageCode:    "en-US",
				},
				SingleUtterance: false,
				InterimResults:  true,
			},
		},
	}); err != nil {
		log.Fatal(err)
	}

	go func() {
		sl := log.New(os.Stderr, "", 0)
		sl.Println("start sending to Speech API...")
		// Pipe stdin to the API.
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if err == io.EOF {
				// Nothing else to pipe, close the stream.
				if err := stream.CloseSend(); err != nil {
					sl.Fatalf("Could not close stream: %v", err)
				}
				return
			}
			if err != nil {
				sl.Printf("Could not read from stdin: %v", err)
				continue
			}

			if err = stream.Send(&speechpb.StreamingRecognizeRequest{
				StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
					AudioContent: buf[:n],
				},
			}); err != nil {
				sl.Printf("Could not send audio: %v", err)
			}
		}
	}()

	rl := log.New(os.Stderr, "", 0)

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Cannot stream results: %v", err)
		}
		if err := resp.Error; err != nil {
			log.Fatalf("Could not recognize: %v", err)
		}
		for _, result := range resp.Results {
			if result.IsFinal {
				for _, alternate := range result.Alternatives {
					rl.Printf("\n\nGOT: { %s } ,\ncorrect= %f %%\n\n", alternate.Transcript, alternate.Confidence)
				}

				continue
			}
			rl.Printf("%s receive= %+v\n", time.Now().Format(time.RFC850), result)
		}
	}
	// [END speech_streaming_mic_recognize]
}
