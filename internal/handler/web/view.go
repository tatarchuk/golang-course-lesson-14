package web

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"homework/internal/auth"
	"homework/internal/model"
	"homework/internal/repository"
)

// pageData is what every template receives. The field names are the contract with the
// templates (prompts.txt, prompt 2.1).
type pageData struct {
	Title     string     // the page title block
	User      *userView  // the signed-in account, nil for an anonymous visitor
	CanDelete bool       // true for staff and admin: renders the delete buttons
	Query     string     // the current search text, echoed into the input
	Albums    []albumRow // the catalogue rows
	Error     string     // the login page's message after a failed attempt
}

// userView is what the layout shows about the signed-in account.
type userView struct {
	Email string
	Role  string
}

// albumRow is one row of the catalogue table. It is a view model of this package: the
// API's model.Album keeps its relations, the row flattens them for the template.
type albumRow struct {
	ID          int64
	Title       string
	Artists     string // artist names joined with ", ", in id order
	Genre       string
	ReleaseYear *int // nil renders an empty cell
	Label       string
	Available   int
	Total       int
	CanDelete   bool
}

// visitor is who is looking at the page, resolved from the claims CookieAuth put in the
// context; the zero value is an anonymous visitor.
type visitor struct {
	user      *userView
	canDelete bool
}

// visitor resolves the account behind the request. The role comes from the token, as it
// does for RequireRole on the delete route; the e-mail comes from the database, and a
// token whose account no longer exists renders as anonymous.
func (h *Handler) visitor(r *http.Request) (visitor, error) {
	claims, ok := auth.UserFromContext(r.Context())
	if !ok {
		return visitor{}, nil
	}
	account, err := h.users.Get(r.Context(), claims.UserID)
	if errors.Is(err, repository.ErrNotFound) {
		return visitor{}, nil
	}
	if err != nil {
		return visitor{}, err
	}
	return visitor{
		user:      &userView{Email: account.Email, Role: claims.Role},
		canDelete: claims.Role == model.UserRoleStaff || claims.Role == model.UserRoleAdmin,
	}, nil
}

// rows loads one page of the catalogue (the first pageLimit matches of query) and the
// availability of every row with two repository calls: one for the albums with their
// relations and one GROUP BY for the copies, however many rows the page holds.
func (h *Handler) rows(ctx context.Context, query string, canDelete bool) ([]albumRow, error) {
	albums, err := h.albums.List(ctx, repository.ListOptions{Query: query, Page: 1, Limit: pageLimit})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(albums))
	for _, a := range albums {
		ids = append(ids, a.ID)
	}
	availability, err := h.copies.AvailabilityForAlbums(ctx, ids)
	if err != nil {
		return nil, err
	}
	rows := make([]albumRow, 0, len(albums))
	for _, a := range albums {
		av := availability[a.ID]
		rows = append(rows, albumRow{
			ID:          a.ID,
			Title:       a.Title,
			Artists:     artistNames(a.Artists),
			Genre:       a.Genre.Name,
			ReleaseYear: a.ReleaseYear,
			Label:       a.Label,
			Available:   int(av.AvailableCopies),
			Total:       int(av.TotalCopies),
			CanDelete:   canDelete,
		})
	}
	return rows, nil
}

func artistNames(artists []model.Artist) string {
	names := make([]string, 0, len(artists))
	for _, a := range artists {
		names = append(names, a.Name)
	}
	return strings.Join(names, ", ")
}
