package api

import (
	"fmt"
	"net/http"
	"strconv"

	"doo/internal/export"
)

func (s *Server) handleExportCall(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid call id")
		return
	}

	call, err := s.Store.GetCall(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if call == nil {
		writeError(w, http.StatusNotFound, "call not found")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "curl"
	}

	var output string
	switch format {
	case "curl":
		output, err = export.ToCurl(call.TargetSnapshotJSON, call.RequestJSON)
	case "python":
		output, err = export.ToPython(call.TargetSnapshotJSON, call.RequestJSON)
	default:
		writeError(w, http.StatusBadRequest, "format must be curl or python")
		return
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("export failed: %v", err))
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(output))
}
