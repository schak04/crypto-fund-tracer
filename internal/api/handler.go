package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/schak04/crypto-fund-tracer/internal/apperror"
	"github.com/schak04/crypto-fund-tracer/internal/service"
	"github.com/schak04/crypto-fund-tracer/internal/validator"
)

// handler manages HTTP endpoints for the investigation REST API
type Handler struct {
	svc service.Service
}

func NewHandler(svc service.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/investigations", h.CreateInvestigation)
	mux.HandleFunc("GET /api/v1/investigations/{id}", h.GetInvestigation)
	return mux
}

func (h *Handler) CreateInvestigation(w http.ResponseWriter, r *http.Request) {
	// require explicit application/json header per API specification before parsing body
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		_ = apperror.WriteJSON(w, apperror.NewInvalidRequest("Content-Type header must be application/json."))
		return
	}

	var req validator.CreateInvestigationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		_ = apperror.WriteJSON(w, apperror.NewInvalidRequest("Request payload is malformed JSON or unparseable."))
		return
	}

	if err := validator.ValidateCreateInvestigationRequest(req); err != nil {
		_ = apperror.WriteJSON(w, err)
		return
	}

	inv, err := h.svc.CreateInvestigation(r.Context(), req.WalletAddress)
	if err != nil {
		_ = apperror.WriteJSON(w, err)
		return
	}

	details, err := h.svc.GetInvestigation(r.Context(), inv.ID)
	if err != nil {
		_ = apperror.WriteJSON(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/investigations/"+details.ID)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(details)
}

func (h *Handler) GetInvestigation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validator.ValidateInvestigationID(id); err != nil {
		_ = apperror.WriteJSON(w, err)
		return
	}

	details, err := h.svc.GetInvestigation(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			_ = apperror.WriteJSON(w, apperror.NewNotFound(fmt.Sprintf("Investigation with ID '%s' does not exist.", id)))
			return
		}
		_ = apperror.WriteJSON(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(details)
}
