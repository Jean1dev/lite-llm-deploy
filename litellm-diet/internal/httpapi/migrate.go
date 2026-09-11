package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/migrate"
)

const migrateTimeout = 2 * time.Minute

type keyImporter interface {
	Import(ctx context.Context, dryRun bool) (migrate.Report, error)
}

func (s *Server) migrateKeys(w http.ResponseWriter, r *http.Request) {
	if !s.requireMaster(w, r) {
		return
	}

	if s.sourceDatabaseURL == "" || s.importer == nil {
		writeError(w, http.StatusServiceUnavailable, "invalid_request_error", "503", "source database not configured")
		return
	}

	if !s.migrateMu.TryLock() {
		writeError(w, http.StatusConflict, "invalid_request_error", "409", "migration already running")
		return
	}
	defer s.migrateMu.Unlock()

	dryRun := r.URL.Query().Get("dry_run") == "true"
	ctx, cancel := context.WithTimeout(r.Context(), migrateTimeout)
	defer cancel()

	rep, err := s.importer.Import(ctx, dryRun)
	if err != nil {
		if s.log != nil {
			s.log.Error("key migration failed", slog.String("err", err.Error()))
		}
		writeError(w, http.StatusBadGateway, "internal_error", "502", "migration failed")
		return
	}

	if !dryRun {
		if err := s.keys.Load(ctx); err != nil {
			if s.log != nil {
				s.log.Error("reload keys after migration", slog.String("err", err.Error()))
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "500", "keys imported but reload failed")
			return
		}
	}

	writeJSON(w, http.StatusOK, rep)
}
