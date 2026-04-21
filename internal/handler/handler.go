package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/aminsalami/funpokedex/internal/domain"
	"github.com/aminsalami/funpokedex/internal/service"
)

type Handler struct {
	svc *service.PokemonService
}

func NewHandler(svc *service.PokemonService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) GetPokemon(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	poke, err := h.svc.GetPokemon(r.Context(), name)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, pokemonResponse{
		Name:        poke.Name,
		Description: poke.Description,
		Habitat:     poke.Habitat,
		IsLegendary: poke.IsLegendary,
	})
}

func (h *Handler) GetTranslatedPokemon(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	poke, err := h.svc.GetTranslatedPokemon(r.Context(), name)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, pokemonResponse{
		Name:        poke.Name,
		Description: poke.Description,
		Habitat:     poke.Habitat,
		IsLegendary: poke.IsLegendary,
	})
}

type pokemonResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Habitat     string `json:"habitat"`
	IsLegendary bool   `json:"isLegendary"`
}

type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		writeJSON(w, appErr.HTTPStatus, errorResponse{
			Error: appErr.Message,
			Code:  appErr.Code,
		})
		return
	}

	writeJSON(w, http.StatusInternalServerError, errorResponse{
		Error: "internal server error",
		Code:  "INTERNAL_ERROR",
	})
}
