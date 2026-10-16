package web

import (
	"net/http"
	"net/url"
	"strings"

	"homework/internal/handler"
)

// Catalog handles GET /catalog: the whole page, filtered by ?q= on the server, so the
// search works without JavaScript too.
func (h *Handler) Catalog(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	who, err := h.visitor(r)
	if err != nil {
		serverError(w, err)
		return
	}
	rows, err := h.rows(r.Context(), query, who.canDelete)
	if err != nil {
		serverError(w, err)
		return
	}
	h.views.renderPage(w, http.StatusOK, pageCatalog, pageData{
		Title:     "Catalogue",
		User:      who.user,
		CanDelete: who.canDelete,
		Query:     query,
		Albums:    rows,
	})
}

// Rows handles GET /web/albums, the live search. For htmx (HX-Request: true) it answers
// the table rows only, which htmx swaps into #album-list; a plain browser request is
// sent to the full page instead, so the address bar never shows a bare fragment.
func (h *Handler) Rows(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if r.Header.Get("HX-Request") != "true" {
		target := "/catalog"
		if query != "" {
			target += "?q=" + url.QueryEscape(query)
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	}
	who, err := h.visitor(r)
	if err != nil {
		serverError(w, err)
		return
	}
	rows, err := h.rows(r.Context(), query, who.canDelete)
	if err != nil {
		serverError(w, err)
		return
	}
	h.views.renderRows(w, pageData{Albums: rows, CanDelete: who.canDelete})
}

// Delete handles DELETE /web/albums/{id} for staff and admin (RequireRole runs first).
// It answers an empty 200, not the API's 204: htmx does not swap on a 204, while an
// empty 200 with hx-swap="outerHTML" removes the row from the table.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := handler.PathID(w, r)
	if !ok {
		return
	}
	if err := h.albums.Delete(r.Context(), id); err != nil {
		handler.WriteRepoError(w, err, "Album")
		return
	}
	w.WriteHeader(http.StatusOK)
}
