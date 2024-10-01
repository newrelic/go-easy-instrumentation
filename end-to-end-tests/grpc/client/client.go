// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"

	"grpc-app/sampleapp"

	"google.golang.org/grpc"
)

func doUnaryUnary(ctx context.Context, client sampleapp.SampleApplicationClient) {
	msg, err := client.DoUnaryUnary(ctx, &sampleapp.Message{Text: "Hello DoUnaryUnary"})
	if err != nil {
		panic(err)
	}
	fmt.Println(msg.Text)
}

func doUnaryStream(ctx context.Context, client sampleapp.SampleApplicationClient) {
	stream, err := client.DoUnaryStream(ctx, &sampleapp.Message{Text: "Hello DoUnaryStream"})
	if err != nil {
		panic(err)
	}
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		fmt.Println(msg.Text)
	}
}

func doStreamUnary(ctx context.Context, client sampleapp.SampleApplicationClient) {
	stream, err := client.DoStreamUnary(ctx)
	if err != nil {
		panic(err)
	}
	for i := 0; i < 3; i++ {
		if err := stream.Send(&sampleapp.Message{Text: "Hello DoStreamUnary"}); err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}
	}
	msg, err := stream.CloseAndRecv()
	if err != nil {
		panic(err)
	}
	fmt.Println(msg.Text)
}

func doStreamStream(ctx context.Context, client sampleapp.SampleApplicationClient) {
	stream, err := client.DoStreamStream(ctx)
	if err != nil {
		panic(err)
	}
	waitc := make(chan struct{})
	go func() {
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				close(waitc)
				return
			}
			if err != nil {
				panic(err)
			}
			fmt.Println(msg.Text)
		}
	}()
	for i := 0; i < 3; i++ {
		if err := stream.Send(&sampleapp.Message{Text: "Hello DoStreamStream"}); err != nil {
			panic(err)
		}
	}
	stream.CloseSend()
	<-waitc
}

func doClientCalls(ctx context.Context, client sampleapp.SampleApplicationClient) {
	doUnaryUnary(ctx, client)
	doUnaryStream(ctx, client)
	doStreamUnary(ctx, client)
	doStreamStream(ctx, client)
}

func main() {
	conn, err := grpc.Dial(
		"localhost:8080",
	)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := sampleapp.NewSampleApplicationClient(conn)
	ctx := context.Background()

	doClientCalls(ctx, client)
}
