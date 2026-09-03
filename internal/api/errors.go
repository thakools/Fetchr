package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"

	"github.com/local/sftpweb/internal/sftpconn"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write response", "err", err)
	}
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, apiError{Code: code, Message: msg})
}

// writeSFTPErr maps remote filesystem failures onto HTTP status codes.
func writeSFTPErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sftpconn.ErrNotConnected):
		writeErr(w, http.StatusConflict, "not_connected", "no active SFTP connection")
	case errors.Is(err, sftpconn.ErrHostDenied):
		writeErr(w, http.StatusForbidden, "host_denied", "this SFTP host is not on the allowlist")
	case errors.Is(err, sftpconn.ErrPoolFull):
		writeErr(w, http.StatusServiceUnavailable, "pool_full", "server connection limit reached, try again later")
	case errors.Is(err, ErrBadPath):
		writeErr(w, http.StatusBadRequest, "bad_path", "invalid path")
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, os.ErrNotExist):
		writeErr(w, http.StatusNotFound, "not_found", "no such file or directory")
	case errors.Is(err, fs.ErrPermission), errors.Is(err, os.ErrPermission):
		writeErr(w, http.StatusForbidden, "permission_denied", "the SFTP server denied this operation")
	case errors.Is(err, fs.ErrExist):
		writeErr(w, http.StatusConflict, "exists", "a file with that name already exists")
	default:
		slog.Warn("sftp operation failed", "err", err)
		writeErr(w, http.StatusBadGateway, "sftp_error", "the SFTP server rejected the request")
	}
}
