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

	"github.com/jobasfernandes/whaticket-go-backend/internal/auth"
	"github.com/jobasfernandes/whaticket-go-backend/internal/contact"
	"github.com/jobasfernandes/whaticket-go-backend/internal/message"
	"github.com/jobasfernandes/whaticket-go-backend/internal/queue"
	"github.com/jobasfernandes/whaticket-go-backend/internal/quickanswer"
	"github.com/jobasfernandes/whaticket-go-backend/internal/rmq"
	"github.com/jobasfernandes/whaticket-go-backend/internal/setting"
	"github.com/jobasfernandes/whaticket-go-backend/internal/ticket"
	"github.com/jobasfernandes/whaticket-go-backend/internal/user"
	"github.com/jobasfernandes/whaticket-go-backend/internal/waevents"
	"github.com/jobasfernandes/whaticket-go-backend/internal/whatsapp"
	"github.com/jobasfernandes/whaticket-go-backend/internal/ws"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fallback := slog.New(slog.NewJSONHandler(os.Stderr, nil))
		fallback.Error("config load failed", "err", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	db, err := openDB(cfg.DatabaseDSN)
	if err != nil {
		logger.Error("db connect failed", "err", err)
		os.Exit(1)
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

	messageTicketSvc := newMessageTicketAdapter(ticketDeps)
	messageDeps := &message.Deps{DB: db, WS: wsHub, TicketSvc: messageTicketSvc}
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
		MessageSvc:  newWaeventsMessageAdapter(messageDeps),
		Log:         logger,
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestID, middleware.Recoverer)
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
	go func() {
		logger.Info("http listening", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server crashed", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

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
}
