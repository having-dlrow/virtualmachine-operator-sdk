package server

import (
	"context"
	vmv1 "github.com/example/virtualmachine/api/v1"
	_ "github.com/example/virtualmachine/docs"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	echoSwagger "github.com/swaggo/echo-swagger"
	"k8s.io/apimachinery/pkg/runtime"
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
	echoLog = ctrl.Log.WithName("echo")
)

type Server struct {
	k8sClient client.Client
}

func (s Server) Init() {

	//Initialize Kubernetes client
	cfg, err := config.GetConfig()
	if err != nil {
		echoLog.Error(err, "Failed to get Kubernetes config")
	}

	// Register the VirtualMachine type with the scheme
	scheme := runtime.NewScheme()
	if err := vmv1.AddToScheme(scheme); err != nil {
		echoLog.Error(err, "Failed to add VirtualMachine to scheme")
		os.Exit(1)
	}

	s.k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		echoLog.Error(err, "Failed to create Kubernetes client")
		os.Exit(1)
	}

	// Initialize Echo
	e := echo.New()
	e.Logger.SetLevel(log.INFO)
	e.Use(middleware.Logger()) // Middleware to log requests

	// swagger
	e.GET("/swagger/*", echoSwagger.WrapHandler)

	echoLog.Info("echo server started")

	e.Logger.Printf("echo logger")

	// Fetch the list of VirtualMachines
	vmList := &vmv1.VirtualMachineList{}
	err = s.k8sClient.List(context.Background(), vmList, &client.ListOptions{
		Namespace: "default", // Replace with the desired namespace or leave empty to list all namespaces
	})
	if err != nil {
		echoLog.Error(err, "Failed to list VirtualMachines")
	} else {
		// Iterate over the list and print the names of the VirtualMachines
		for _, vm := range vmList.Items {
			echoLog.Info("VirtualMachine", "name", vm.Name, "namespace", vm.Namespace, "status", vm.Status)
		}
	}

	s.InitRoutes(e)

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
