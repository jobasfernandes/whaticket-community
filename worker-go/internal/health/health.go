package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jobasfernandes/whaticket-go-worker/internal/media"
	"github.com/jobasfernandes/whaticket-go-worker/internal/rmq"
	whatsmeowpkg "github.com/jobasfernandes/whaticket-go-worker/internal/whatsmeow"
)

const defaultHealthTimeout = 3 * time.Second

type Server struct {
	*http.Server
	rmq   *rmq.Client
	minio *media.MinioClient
	mgr   *whatsmeowpkg.Manager
}

func New(rmqClient *rmq.Client, minioClient *media.MinioClient, mgr *whatsmeowpkg.Manager, port string) *Server {
	if port == "" {
		port = "8081"
	}
	s := &Server{rmq: rmqClient, minio: minioClient, mgr: mgr}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handle)
	s.Server = &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultHealthTimeout)
	defer cancel()

	rmqOK := s.rmq != nil && s.rmq.IsConnected()
	minioOK := false
	if s.minio != nil {
		minioOK = s.minio.Health(ctx) == nil
	}

	sessions := 0
	if s.mgr != nil {
		sessions = s.mgr.Count()
	}

	status := "ok"
	httpStatus := http.StatusOK
	if !rmqOK || !minioOK {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	body := map[string]any{
		"status":   status,
		"rmq":      rmqOK,
		"minio":    minioOK,
		"sessions": sessions,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(body)
}
