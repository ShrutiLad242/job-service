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

	log.Printf("[Main] Starting job-handler service")

	store := store.NewJobStore() //this store will contain all the jobs
	log.Printf("[Main] JobStore initialized")

	workerPool := worker.NewWorkerPool(3, store)
	log.Printf("[Main] WorkerPool created with 3 workers")
	workerPool.Start() //3 go routines will start
	log.Printf("[Main] WorkerPool started")

	handler := handler.NewJobHandler(store, workerPool)
	log.Printf("[Main] JobHandler initialized")

	router := gin.Default()
	log.Printf("[Main] Gin router initialized")

	router.POST("/jobs", handler.CreateJob)
	router.GET("/jobs/:id", handler.GetJob)
	router.GET("/jobs", handler.ListJobs)
	router.POST("/jobs/:id/cancel", handler.CancelJob)
	log.Printf("[Main] All HTTP routes registered")

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		log.Println("[Main] Server started on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Main] listen: %s\n", err)
		}
	}()

	// Listen for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Println("[Main] Shutdown signal received")

	// Stop accepting new requests
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[Main] Server shutdown error:%+v\n", err)
	}
	log.Printf("[Main] HTTP server shutdown complete")

	// Shutdown worker pool
	log.Printf("[Main] Shutting down worker pool")
	workerPool.Shutdown()

	log.Println("[Main] Server exited properly")
}
