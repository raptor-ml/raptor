/*
Copyright (c) 2022 Raptor.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"github.com/go-logr/zapr"
	grpcMiddleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpcZap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpcRetry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	grpcCtxTags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpcValidator "github.com/grpc-ecosystem/go-grpc-middleware/validator"
	"github.com/raptor-ml/raptor/internal/programregistry"
	"github.com/raptor-ml/raptor/internal/runtime"
	"github.com/raptor-ml/raptor/internal/version"
	"github.com/raptor-ml/raptor/pkg/sdk"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	pbEngine "go.buf.build/raptor/api-go/raptor/core/raptor/core/v1alpha1"
	pbRuntime "go.buf.build/raptor/api-go/raptor/core/raptor/runtime/v1alpha1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials/local"
	"google.golang.org/grpc/reflection"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	pflag.Bool("production", true, "Set as production")
	pflag.String("core-grpc-url", "core.raptor-system:60000", "The gRPC URL of the Raptor's Core")
	pflag.String("grpc-addr", ":60005", "The gRPC address to listen on")
	pflag.Parse()
	must(viper.BindPFlags(pflag.CommandLine))

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.AutomaticEnv()

	zl := logger()
	logger := zapr.NewLogger(zl)

	logger.WithName("setup").WithValues("version", version.Version).Info("Initializing Raptor Runtime...")

	// Creating Engine
	cc, err := grpc.Dial(
		viper.GetString("core-grpc-url"),
		grpc.WithStreamInterceptor(grpcMiddleware.ChainStreamClient(
			grpcZap.StreamClientInterceptor(zl.Named("core")),
			grpcRetry.StreamClientInterceptor(),
		)),
		grpc.WithUnaryInterceptor(grpcMiddleware.ChainUnaryClient(
			grpcZap.UnaryClientInterceptor(zl.Named("core")),
			grpcRetry.UnaryClientInterceptor(),
		)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	must(err)
	engine := sdk.NewGRPCEngine(pbEngine.NewEngineServiceClient(cc))

	// Creating app context
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Creating PyExp program registry
	programs := programregistry.New(ctx, engine)

	// Creating runtime implementation
	rt := runtime.New(engine, programs, logger.WithName("runtime"))

	// Creating gRPC server
	server := grpc.NewServer(
		grpc.StreamInterceptor(grpcMiddleware.ChainStreamServer(
			grpcCtxTags.StreamServerInterceptor(),
			grpcZap.StreamServerInterceptor(zl.Named("grpc.runtime")),
			grpcValidator.StreamServerInterceptor(),
		)),
		grpc.UnaryInterceptor(grpcMiddleware.ChainUnaryServer(
			grpcCtxTags.UnaryServerInterceptor(),
			grpcZap.UnaryServerInterceptor(zl.Named("grpc.runtime")),
			grpcValidator.UnaryServerInterceptor(),
		)),
		grpc.Creds(local.NewCredentials()),
	)
	pbEngine.RegisterEngineServiceServer(server, sdk.NewServiceServer(engine))
	pbRuntime.RegisterRuntimeServiceServer(server, rt)
	reflection.Register(server)

	l, err := net.Listen("tcp", viper.GetString("grpc-addr"))
	must(err)

	logger.WithValues("kind", "grpc", "addr", l.Addr()).Info("Starting Runtime server")
	go func() {
		<-ctx.Done()
		server.Stop()
	}()

	must(server.Serve(l))
}
func logger() *zap.Logger {
	var l *zap.Logger
	var err error
	if viper.GetBool("production") {
		l, err = zap.NewProduction()
	} else {
		l, err = zap.NewDevelopment()
	}
	must(err)

	return l
}

func must(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
