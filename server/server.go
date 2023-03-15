package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"cloud.google.com/go/compute/metadata"
	texporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	stdout "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"

	"github.com/gabihodoroga/pubsub-to-bigquery/config"
	"github.com/gabihodoroga/pubsub-to-bigquery/service"
)

func Start(logger *zap.Logger, loggerLevel zap.AtomicLevel) {

	// Setup tracer
	onGCE := metadata.OnGCE()
	if onGCE {
		// infer our project id from the metadata server
		projectID, err := metadata.ProjectID()
		if err != nil {
			fmt.Printf("metadata.ProjectID(): %v", err)
			os.Exit(1)
			return
		}
		// init open telemetry, while allowing us to defer the teardown and flushing of our exporter
		tracerShutdown, err := initTracer(context.Background(), projectID, onGCE)
		if err != nil {
			fmt.Printf("initTracer() error: %v", err)
			os.Exit(1)
			return
		}
		defer tracerShutdown()
	}

	cfg := config.GetConfig()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// setup the required services
	sink, err := service.NewEventSinkBigQuery(ctx, cfg.BigQueryProject, cfg.BigQueryDataset, cfg.BigQueryTable)
	if err != nil {
		fmt.Printf("failed to create EventSinkBigQuery: %v", err)
		os.Exit(1)
		return
	}

	handler, err := service.NewEventHandlerPubsub("", cfg.PubsubProject, cfg.PubSubSubscription, sink)
	if err != nil {
		fmt.Printf("failed to create EventHandlerPubsub: %v", err)
		os.Exit(1)
		return
	}

	err = handler.Start(ctx)
	if err != nil {
		fmt.Printf("failed to start EventHandlerPubsub: %v", err)
		os.Exit(1)
		return
	}

	r := gin.New()
	r.Use(otelgin.Middleware("gin-router"))
	// setup you routes here
	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.GET("/stats", func(c *gin.Context) {
		stats, err := handler.Stats(c.Request.Context())
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		c.JSON(http.StatusOK, stats)
	})
	// setup the http server
	srv := &http.Server{
		Addr:    resolveAddress(),
		Handler: r,
	}

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		logger.Sugar().Infof("httpServer: listen on address %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("listen: %s\n", err)
			os.Exit(1)
		}
	}()

	// Listen for the interrupt signal.
	<-ctx.Done()

	// stop() must be called here to restore default behavior
	// on the interrupt signal and notify user of shutdown,
	// otherwise user will not be able to stop with ctrl+c anymore.
	stop()
	logger.Info("httpServer: shutting down gracefully...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		fmt.Printf("server forced to shutdown: %v\n", err)
		os.Exit(1)
	}
	logger.Info("httpServer: server exiting...")

}

func resolveAddress() string {
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}
	return ":8080"
}

func initTracer(ctx context.Context, projectID string, onGCE bool) (func(), error) {
	var tpOpts []sdktrace.TracerProviderOption
	// if our code is running on GCP (cloud run/functions/app engine/gke) then we will export directly to cloud tracing
	if onGCE {
		exporter, err := texporter.New(texporter.WithProjectID(projectID))
		if err != nil {
			return nil, errors.Wrap(err, "initTracer: texporter.New()")
		}
		tpOpts = append(tpOpts, sdktrace.WithBatcher(exporter))
		if config.GetConfig().TraceSample != "" {
			sampleRate, err := strconv.ParseFloat(config.GetConfig().TraceSample, 64)
			if err != nil {
				return nil, errors.Wrap(err, "initTracer: invalid trace sample rate")
			}
			tpOpts = append(tpOpts, sdktrace.WithSampler(sdktrace.TraceIDRatioBased(sampleRate)))
		}
	} else {
		exporter, err := stdout.New(stdout.WithPrettyPrint())
		if err != nil {
			return nil, errors.Wrapf(err, "initTracer: stdout.New()")
		}
		tpOpts = append(tpOpts, sdktrace.WithBatcher(exporter))
	}

	tp := sdktrace.NewTracerProvider(tpOpts...)

	otel.SetTracerProvider(tp)

	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(propagator)

	return func() {
		err := tp.ForceFlush(ctx)
		if err != nil {
			fmt.Printf("tracerProvider.ForceFlush() error: %v\n", err)
		}
	}, nil
}
