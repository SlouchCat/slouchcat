package admin

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Limits of the admin item search (contract searchAdminItems).
const (
	minItemSearchQuery = 2
	maxItemSearchQuery = 100
	defaultSearchLimit = 20
	maxSearchLimit     = 50
)

// SearchItems finds titles by name for the pick and collection dialogs, without any profile filter
// (the System scope of the visibility layer). limit 0 means the default of 20.
func (s *Service) SearchItems(ctx context.Context, query string, limit int) ([]model.AdminItemHit, error) {
	query = strings.TrimSpace(query)
	var verr *model.ValidationError
	switch n := utf8.RuneCountInString(query); {
	case n < minItemSearchQuery:
		verr = verr.Add("q", "too_short")
	case n > maxItemSearchQuery:
		verr = verr.Add("q", "too_long")
	}
	if limit == 0 {
		limit = defaultSearchLimit
	}
	if limit < 1 || limit > maxSearchLimit {
		verr = verr.Add("limit", "invalid_value")
	}
	if verr != nil {
		return nil, verr
	}
	return s.vis.AdminSearch(ctx, visibility.System("admin item search"), query, limit)
}
