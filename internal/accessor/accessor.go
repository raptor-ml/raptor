package accessor

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_validator "github.com/grpc-ecosystem/go-grpc-middleware/validator"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/natun-ai/natun/pkg/api"
	"github.com/natun-ai/natun/pkg/sdk"
	coreApi "github.com/natun-ai/natun/proto/gen/go/natun/core/v1alpha1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

type Accessor interface {
	Grpc(addr string) NoLeaderRunnableFunc
	Http(addr string, prefix string) NoLeaderRunnableFunc
}

type accessor struct {
	sdkServer coreApi.EngineServiceServer
	server    *grpc.Server
	logger    logr.Logger
}

func New(e api.Manager) Accessor {
	svc := &accessor{
		sdkServer: sdk.NewServiceServer(e),
		logger:    e.Logger().WithName("accessor"),
	}

	zapLogger := svc.logger.GetSink().(zapr.Underlier).GetUnderlying()

	grpcMetrics := grpc_prometheus.NewServerMetrics()
	metrics.Registry.MustRegister(grpcMetrics)

	svc.server = grpc.NewServer(
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
			grpc_ctxtags.StreamServerInterceptor(),
			grpcMetrics.StreamServerInterceptor(),
			grpc_zap.StreamServerInterceptor(zapLogger),
			grpc_validator.StreamServerInterceptor(),
		)),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			grpc_ctxtags.UnaryServerInterceptor(),
			grpcMetrics.UnaryServerInterceptor(),
			grpc_zap.UnaryServerInterceptor(zapLogger),
			grpc_validator.UnaryServerInterceptor(),
		)),
	)
	coreApi.RegisterEngineServiceServer(svc.server, svc.sdkServer)
	grpcMetrics.InitializeMetrics(svc.server)
	reflection.Register(svc.server)

	return svc
}

func (a *accessor) Grpc(addr string) NoLeaderRunnableFunc {
	return func(ctx context.Context) error {
		l, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("failed to listen: %w", err)
		}

		a.logger.WithValues("kind", "grpc", "addr", l.Addr()).Info("Starting Accessor server")
		go func() {
			<-ctx.Done()
			a.server.Stop()
		}()
		return a.server.Serve(l)
	}
}

func (a *accessor) Http(addr string, prefix string) NoLeaderRunnableFunc {
	return func(ctx context.Context) error {
		gwMux := runtime.NewServeMux()
		err := coreApi.RegisterEngineServiceHandlerServer(ctx, gwMux, a.sdkServer)
		if err != nil {
			return fmt.Errorf("failed to register grpc gateway: %w", err)
		}

		if prefix[len(prefix)-1] == '/' {
			prefix += "/"
		}
		mux := http.NewServeMux()
		mux.Handle(prefix[:len(prefix)-1], http.StripPrefix(fmt.Sprintf("%s/", prefix), gwMux))

		l, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("failed to listen: %w", err)
		}

		a.logger.WithValues("kind", "http", "addr", l.Addr).Info("Starting Accessor server")
		go func() {
			<-ctx.Done()
			_ = l.Close()
		}()
		return http.Serve(l, mux)
	}
}
