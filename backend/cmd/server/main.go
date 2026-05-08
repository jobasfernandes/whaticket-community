package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/canove/whaticket-community/backend/internal/auth"
	"github.com/canove/whaticket-community/backend/internal/contact"
	"github.com/canove/whaticket-community/backend/internal/dbmigrate"
	"github.com/canove/whaticket-community/backend/internal/dbseed"
	"github.com/canove/whaticket-community/backend/internal/media"
	"github.com/canove/whaticket-community/backend/internal/message"
	"github.com/canove/whaticket-community/backend/internal/queue"
	"github.com/canove/whaticket-community/backend/internal/quickanswer"
	"github.com/canove/whaticket-community/backend/internal/rmq"
	"github.com/canove/whaticket-community/backend/internal/setting"
	"github.com/canove/whaticket-community/backend/internal/ticket"
	"github.com/canove/whaticket-community/backend/internal/user"
	"github.com/canove/whaticket-community/backend/internal/waevents"
	"github.com/canove/whaticket-community/backend/internal/whatsapp"
	"github.com/canove/whaticket-community/backend/internal/ws"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fallback := slog.New(slog.NewJSONHandler(os.Stderr, nil))
		fallback.Error("config load failed", "err", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	if err := Run(ctx, cfg); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}

func Run(ctx context.Context, cfg appConfig) error {
	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	if cfg.AutoMigrate {
		if _, err := dbmigrate.Up(ctx, cfg.DatabaseDSN, logger); err != nil {
			return err
		}
	}

	db, err := openDB(cfg.DatabaseDSN)
	if err != nil {
		return err
	}

	if cfg.AutoSeed {
		if _, err := dbseed.Run(ctx, db, dbseed.Options{
			AdminEmail:    cfg.SeedEmail,
			AdminName:     cfg.SeedName,
			AdminPassword: cfg.SeedPassword,
		}, logger); err != nil {
			return err
		}
	}

	rmqClient := rmq.New(rmq.Config{URL: cfg.RMQURL, Logger: logger})
	rmqClient.SetRole(rmq.RoleBackend)
	if err := rmqClient.Start(ctx); err != nil {
		logger.Warn("rmq start failed; continuing degraded", "err", err)
	}

	wsHub := ws.NewHub(ws.Config{
		JWTSecret:     []byte(cfg.AccessSecret),
		AllowedOrigin: cfg.FrontendURL,
		Logger:        logger,
	})

	authLoader := user.NewAuthLoader(db)
	settingChecker := setting.NewSettingChecker(db)

	authDeps := &auth.Deps{
		DB:            db,
		Loader:        authLoader,
		AccessSecret:  []byte(cfg.AccessSecret),
		RefreshSecret: []byte(cfg.RefreshSecret),
	}

	userDeps := &user.Deps{DB: db, WS: wsHub, Settings: settingChecker}
	userHandler := &user.Handler{Deps: userDeps}

	queueDeps := &queue.Deps{DB: db, WS: wsHub}
	queueHandler := &queue.Handler{Deps: queueDeps}

	settingDeps := &setting.Deps{DB: db, WS: wsHub}
	settingHandler := &setting.Handler{Deps: settingDeps}

	quickAnswerDeps := &quickanswer.Deps{DB: db, WS: wsHub}
	quickAnswerHandler := &quickanswer.Handler{Deps: quickAnswerDeps}

	contactDeps := &contact.Deps{DB: db, WS: wsHub}
	contactHandler := &contact.Handler{Deps: contactDeps, Logger: logger}

	userQueueLookup := user.NewQueueLookup(db)
	ticketDeps := &ticket.Deps{DB: db, WS: wsHub, UserService: userQueueLookup}
	ticketHandler := &ticket.Handler{Deps: ticketDeps, Logger: logger, AccessSecret: []byte(cfg.AccessSecret)}

	wsHub.SetTicketAuthorizer(ticket.NewWSAuthz(ticketDeps))

	mediaUploader := buildMediaUploader(cfg, logger)

	messageTicketSvc := newMessageTicketAdapter(ticketDeps)
	messageDeps := &message.Deps{
		DB:            db,
		WS:            wsHub,
		TicketSvc:     messageTicketSvc,
		Sender:        newMessageSenderAdapter(rmqClient),
		MediaUploader: mediaUploader,
	}
	messageHandler := &message.Handler{Deps: messageDeps, Logger: logger, AccessSecret: []byte(cfg.AccessSecret)}

	whatsappDeps := &whatsapp.Deps{DB: db, WS: wsHub, RMQ: rmqClient, RPC: rmqClient, Logger: logger}
	whatsappHandler := &whatsapp.Handler{Deps: whatsappDeps, Logger: logger, AccessSecret: []byte(cfg.AccessSecret)}

	waeventsConsumer := &waevents.Consumer{
		DB:          db,
		RMQ:         newRMQEnvelopeAdapter(rmqClient),
		RPC:         rmqClient,
		WS:          wsHub,
		WhatsappSvc: newWaeventsWhatsappAdapter(whatsappDeps, rmqClient),
		ContactSvc:  newWaeventsContactAdapter(contactDeps),
		TicketSvc:   newWaeventsTicketAdapter(ticketDeps),
		MessageSvc:  newWaeventsMessageAdapter(messageDeps, ticketDeps),
		Log:         logger,
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestID, middleware.Recoverer)
	router.Use(middleware.StripSlashes)
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins(cfg.FrontendURL),
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	authDeps.Routes(router)
	userHandler.Routes(router, []byte(cfg.AccessSecret))
	userHandler.MountSignup(router)
	queueHandler.Routes(router, []byte(cfg.AccessSecret))
	settingHandler.Routes(router, []byte(cfg.AccessSecret))
	quickAnswerHandler.Routes(router, []byte(cfg.AccessSecret))
	contactHandler.Routes(router, []byte(cfg.AccessSecret))
	ticketHandler.Routes(router)
	messageHandler.Routes(router)
	whatsappHandler.Routes(router)
	router.Get("/ws", wsHub.Handle)

	go func() {
		if err := waeventsConsumer.Start(ctx, rmqClient); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("waevents consumer crashed", "err", err)
		}
	}()

	if err := whatsapp.StartAllSessions(ctx, db, rmqClient, logger); err != nil {
		logger.Warn("StartAllSessions failed", "err", err)
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	httpErrCh := make(chan error, 1)
	go func() {
		logger.Info("http listening", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			httpErrCh <- err
			return
		}
		httpErrCh <- nil
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down")
	case err := <-httpErrCh:
		if err != nil {
			logger.Error("http server crashed", "err", err)
			return err
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("http shutdown", "err", err)
	}
	if err := wsHub.Shutdown(shutdownCtx); err != nil {
		logger.Warn("ws shutdown", "err", err)
	}
	if err := rmqClient.Shutdown(shutdownCtx); err != nil {
		logger.Warn("rmq shutdown", "err", err)
	}
	logger.Info("bye")
	return nil
}

func buildMediaUploader(cfg appConfig, logger *slog.Logger) message.MediaUploader {
	if cfg.MinioEndpoint == "" {
		logger.Warn("BACKEND_S3_ENDPOINT not set, outbound media uploads disabled")
		return media.NoopClient{}
	}
	client, err := media.New(media.Config{
		Endpoint:  cfg.MinioEndpoint,
		AccessKey: cfg.MinioAccessKey,
		SecretKey: cfg.MinioSecretKey,
		Bucket:    cfg.MinioBucket,
		PublicURL: cfg.MinioPublicURL,
		UseSSL:    cfg.MinioUseSSL,
	}, logger)
	if err != nil {
		logger.Warn("media client init failed; falling back to noop", "err", err)
		return media.NoopClient{}
	}
	return client
}
