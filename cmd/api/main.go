package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/drivebai/backend/internal/auth"
	"github.com/drivebai/backend/internal/config"
	"github.com/drivebai/backend/internal/database"
	"github.com/drivebai/backend/internal/handlers"
	"github.com/drivebai/backend/internal/middleware"
	"github.com/drivebai/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	chiMW "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	fmt.Println("DriveBai Backend — Checkpoint 3 Defense")

	// Config
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	// Logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Database
	ctx := context.Background()
	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connect failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	logger.Info("connected to database")

	// Services
	jwtSvc := auth.NewJWTService(cfg.JWTSecret, cfg.JWTAccessTokenTTL, cfg.JWTRefreshTokenTTL)

	// Repositories
	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewTokenRepository(db)
	otpRepo := repository.NewLoginOTPRepository(db)
	carRepo := repository.NewCarRepository(db)
	likesRepo := repository.NewLikesRepository(db)
	docRepo := repository.NewDocumentRepository(db)

	uploadDir := cfg.UploadDir
	if uploadDir == "" {
		uploadDir = "./upload"
	}
	if err := os.MkdirAll(uploadDir, 0775); err != nil {
		logger.Error("failed to create uploads directory", "error", err)
		os.Exit(1)
	}
	// Handlers
	otpAuth := handlers.NewOTPAuthHandler(userRepo, tokenRepo, otpRepo, jwtSvc, logger)
	carHandler := handlers.NewCarHandler(carRepo, userRepo)
	likesHandler := handlers.NewLikesHandler(likesRepo, carRepo)
	userHandler := handlers.NewUserHandler(userRepo, docRepo, logger, uploadDir)

	// Router
	r := chi.NewRouter()
	r.Use(chiMW.RequestID)
	r.Use(chiMW.RealIP)
	r.Use(middleware.Logger(logger))
	r.Use(chiMW.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Health(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("unhealthy"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// API
	r.Route("/api/v1", func(r chi.Router) {
		// Public
		r.Get("/listings", carHandler.ListAvailableListings)

		// Auth (public)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/otp/request", otpAuth.RequestOTP)
			r.Post("/otp/verify", otpAuth.VerifyOTP)
			r.Post("/otp/complete-registration", otpAuth.CompleteRegistration)
			r.Post("/token/refresh", otpAuth.RefreshToken)
		})

		// Protected
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(jwtSvc))
			r.Get("/me", userHandler.GetCurrentUser)

			r.Route("/cars", func(r chi.Router) {
				r.Post("/", carHandler.CreateCar)
				r.Put("/{carId}", carHandler.UpdateCar)
			})

			r.Get("/me/likes", likesHandler.GetLikedListings)
			r.Post("/listings/{listingId}/like", likesHandler.LikeListing)
			r.Delete("/listings/{listingId}/like", likesHandler.UnlikeListing)
		})
	})

	// Start
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		logger.Info("starting server", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}
