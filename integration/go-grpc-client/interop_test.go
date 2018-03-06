// Copyright 2018, OpenCensus Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package interop

import (
	"bytes"
	"log"
	"os"
	"testing"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"

	pb "github.com/census-instrumentation/opencensus-experiments/integration/proto"
)

var setups = []struct {
	name         string
	envAddrKey   string
	fallbackAddr string
}{
	{name: "GoClient-GoServer", envAddrKey: "GO_SERVER_ADDR", fallbackAddr: ":9800"},
	// {name: "GoClient-JavaServer", envName: "JAVA_SERVER_ADDR", fallbackAddr: ":9801"},
	// {name: "GoClient-PythonServer", envName: "PYTHON_SERVER_ADDR", fallbackAddr: ":9802"},
}

func TestInterop(t *testing.T) {
	for _, setup := range setups {
		t.Run(setup.name, func(tt *testing.T) {
			addr := os.Getenv(setup.envAddrKey)
			if addr == "" {
				addr = setup.fallbackAddr
			}
			runInteropTest(tt, addr)
		})
	}
}

func runInteropTest(t *testing.T, addr string) {
	conn, err := grpc.Dial(addr, grpc.WithStatsHandler(new(ocgrpc.ClientHandler)), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("go-gRPC client Dial err: %v", err)
	}

	echoClient := pb.NewEchoServiceClient(conn)

	// 1. Create some tags
	ctx, err := tag.New(context.Background(),
		tag.Insert(mustKey("operation"), "interop-test"),
		tag.Insert(mustKey("project"), "open-census"),
	)
	if err != nil {
		t.Fatalf("tag.New err: %v", err)
	}

	// 2. Create a span with a traceID, spanID
	opts := trace.StartOptions{Sampler: trace.AlwaysSample()}
	clientSpan := trace.NewSpan("gRPC-client-span", nil, opts)
	ctx = trace.WithSpan(ctx, clientSpan)
	inSpanCtx := clientSpan.SpanContext()

	// 3. Send those over and ensure that its response echoes them back
	res, err := echoClient.Echo(ctx, new(pb.EchoRequest))
	if err != nil {
		t.Fatalf("Echo err: %v", err)
	}

	if gti, wti := res.TraceId, inSpanCtx.TraceID[:]; !bytes.Equal(gti, wti) {
		t.Errorf("TraceID:\ngot= (% X)\nwant=(% X)", gti, wti)
	}
	if gsi, wsi := res.SpanId, inSpanCtx.SpanID[:]; !bytes.Equal(gsi, wsi) {
		t.Errorf("SpanID:\ngot= (% X)\nwant=(% X)", gsi, wsi)
	}
}

func mustKey(key string) tag.Key {
	k, err := tag.NewKey(key)
	if err != nil {
		log.Fatalf("tag.NewKey: %q err: %v", key, err)
	}
	return k
}
