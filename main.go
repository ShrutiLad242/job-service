package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"job-handler/handler"
	"job-handler/store"
	"job-handler/worker"
)

func main() {

	store := store.NewJobStore() //this store will contain all the jobs

	workerPool := worker.NewWorkerPool(3, store)
	workerPool.Start()  //3 go routines will start

	handler := handler.NewJobHandler(store, workerPool)

	router := gin.Default()

	router.POST("/jobs", handler.CreateJob)
	router.GET("/jobs/:id", handler.GetJob)
	router.GET("/jobs", handler.ListJobs)
	router.POST("/jobs/:id/cancel", handler.CancelJob)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		log.Println("Server started on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Listen for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit 
	log.Println("Shutdown signal received")

	// Stop accepting new requests
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server Shutdown Failed:%+v\n", err)
	}

	// Shutdown worker pool
	workerPool.Shutdown()

	log.Println("Server exited properly")
}
