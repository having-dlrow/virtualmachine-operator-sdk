package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
)

var (
	echoLog = ctrl.Log.WithName("setup")
)

type Server struct {
	//k8sClient client.Client
}

func (s Server) Init() {

	//Initialize Kubernetes client
	cfg, err := config.GetConfig()
	if err != nil {
		echoLog.Error(err, "Failed to get Kubernetes config")
	}

	_, err = client.New(cfg, client.Options{})
	if err != nil {
		echoLog.Error(err, "Failed to create Kubernetes client")
	}

	// Initialize Echo
	e := echo.New()
	//echoLog.Info("echo server started")

	//vm := &vmv1.VirtualMachine{}
	//k8sClient.Get(context.Background(), namespacedName, vm)
	// Routes
	//s.InitRoutes(e)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start server
	go func() {
		if err := e.Start(":8888"); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds.
	<-ctx.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal("Server forced to shutdown:", err)
	}
	e.Logger.Info("Server exiting")
}
