package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/oklog/ulid/v2"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

const (
	// maxCollectionName and maxCollectionItems follow the contract (CollectionRequest,
	// CollectionItemsRequest).
	maxCollectionName  = 120
	maxCollectionItems = 500

	sourceTMDB = "tmdb"
)

// ListCollections returns every collection, TMDB and manual, by name without members (E-6).
func (s *Service) ListCollections(ctx context.Context) ([]model.AdminCollection, error) {
	rows, err := s.sys.AdminListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}
	out := make([]model.AdminCollection, 0, len(rows))
	for _, r := range rows {
		c := toAdminCollection(systemq.AdminGetCollectionRow(r))
		c.Items = []model.AdminCollectionItem{}
		out = append(out, c)
	}
	return out, nil
}

// GetCollection returns a collection with all its live members in order, unfiltered;
// model.ErrNotFound for an unknown one.
func (s *Service) GetCollection(ctx context.Context, id string) (model.AdminCollection, error) {
	return collectionWithItems(ctx, s.sys, id)
}

// CreateCollection creates an empty manual collection.
func (s *Service) CreateCollection(ctx context.Context, name string) (model.AdminCollection, error) {
	name, verr := validateCollectionName(name)
	if verr != nil {
		return model.AdminCollection{}, verr
	}
	id := ulid.MustNew(ulid.Timestamp(s.now()), ulid.DefaultEntropy()).String()
	q := systemq.New(s.db)
	if err := q.AdminInsertCollection(ctx, systemq.AdminInsertCollectionParams{ID: id, Name: name, Now: s.now().UnixMilli()}); err != nil {
		return model.AdminCollection{}, fmt.Errorf("insert collection: %w", err)
	}
	s.log.InfoContext(ctx, "collection created", "collection_id", id)
	return collectionWithItems(ctx, q, id)
}

// UpdateCollection renames a manual collection; TMDB collections are model.ErrNotEditable.
func (s *Service) UpdateCollection(ctx context.Context, id, name string) (model.AdminCollection, error) {
	tx, q, err := s.editableCollection(ctx, id)
	if err != nil {
		return model.AdminCollection{}, err
	}
	defer func() { _ = tx.Rollback() }()
	name, verr := validateCollectionName(name)
	if verr != nil {
		return model.AdminCollection{}, verr
	}
	if err := q.AdminRenameCollection(ctx, systemq.AdminRenameCollectionParams{ID: id, Name: name, Now: s.now().UnixMilli()}); err != nil {
		return model.AdminCollection{}, fmt.Errorf("rename collection %s: %w", id, err)
	}
	c, err := collectionWithItems(ctx, q, id)
	if err != nil {
		return model.AdminCollection{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.AdminCollection{}, fmt.Errorf("commit: %w", err)
	}
	return c, nil
}

// DeleteCollection deletes a manual collection; its members stay as titles. TMDB collections
// are model.ErrNotEditable.
func (s *Service) DeleteCollection(ctx context.Context, id string) error {
	tx, q, err := s.editableCollection(ctx, id)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := q.AdminDeleteCollection(ctx, id); err != nil {
		return fmt.Errorf("delete collection %s: %w", id, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	s.log.InfoContext(ctx, "collection deleted", "collection_id", id)
	return nil
}

// SetCollectionItems replaces the members of a manual collection with itemIDs in this order.
// Unknown or deleted titles are itemIds unknown_item, seasons, episodes and extras (and
// duplicates) itemIds invalid_value; TMDB collections are model.ErrNotEditable.
func (s *Service) SetCollectionItems(ctx context.Context, id string, itemIDs []string) (model.AdminCollection, error) {
	tx, q, err := s.editableCollection(ctx, id)
	if err != nil {
		return model.AdminCollection{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if len(itemIDs) > maxCollectionItems {
		return model.AdminCollection{}, (&model.ValidationError{}).Add("itemIds", "too_long")
	}
	seen := make(map[string]bool, len(itemIDs))
	for _, itemID := range itemIDs {
		if seen[itemID] {
			return model.AdminCollection{}, (&model.ValidationError{}).Add("itemIds", "invalid_value")
		}
		seen[itemID] = true
		verr, err := s.checkCurationItem(ctx, q, "itemIds", itemID, "season", "episode", "extra")
		if err != nil {
			return model.AdminCollection{}, err
		}
		if verr != nil {
			return model.AdminCollection{}, verr
		}
	}
	now := s.now().UnixMilli()
	if err := q.AdminClearCollectionItems(ctx, id); err != nil {
		return model.AdminCollection{}, fmt.Errorf("clear collection %s: %w", id, err)
	}
	for i, itemID := range itemIDs {
		if err := q.AdminInsertCollectionItem(ctx, systemq.AdminInsertCollectionItemParams{
			CollectionID: id, ItemID: itemID, Position: int64(i), Now: now,
		}); err != nil {
			return model.AdminCollection{}, fmt.Errorf("insert member of %s: %w", id, err)
		}
	}
	if err := q.AdminTouchCollection(ctx, systemq.AdminTouchCollectionParams{ID: id, Now: now}); err != nil {
		return model.AdminCollection{}, fmt.Errorf("touch collection %s: %w", id, err)
	}
	c, err := collectionWithItems(ctx, q, id)
	if err != nil {
		return model.AdminCollection{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.AdminCollection{}, fmt.Errorf("commit: %w", err)
	}
	s.log.InfoContext(ctx, "collection members set", "collection_id", id, "items", len(itemIDs))
	return c, nil
}

// editableCollection opens a write transaction and checks that the collection exists
// (model.ErrNotFound) and is a manual one (model.ErrNotEditable). On success the caller owns the
// transaction.
func (s *Service) editableCollection(ctx context.Context, id string) (*sql.Tx, *systemq.Queries, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin: %w", err)
	}
	q := systemq.New(tx)
	c, err := q.AdminGetCollection(ctx, id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		err = model.ErrNotFound
	case err != nil:
		err = fmt.Errorf("get collection %s: %w", id, err)
	case c.Source == sourceTMDB:
		err = model.ErrNotEditable
	}
	if err != nil {
		_ = tx.Rollback()
		return nil, nil, err
	}
	return tx, q, nil
}

func validateCollectionName(name string) (string, *model.ValidationError) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return "", (&model.ValidationError{}).Add("name", "required")
	case utf8.RuneCountInString(name) > maxCollectionName:
		return "", (&model.ValidationError{}).Add("name", "too_long")
	}
	return name, nil
}

func collectionWithItems(ctx context.Context, q *systemq.Queries, id string) (model.AdminCollection, error) {
	row, err := q.AdminGetCollection(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.AdminCollection{}, model.ErrNotFound
	}
	if err != nil {
		return model.AdminCollection{}, fmt.Errorf("get collection %s: %w", id, err)
	}
	members, err := q.AdminListCollectionItems(ctx, id)
	if err != nil {
		return model.AdminCollection{}, fmt.Errorf("list members of %s: %w", id, err)
	}
	c := toAdminCollection(row)
	c.Items = make([]model.AdminCollectionItem, 0, len(members))
	for _, m := range members {
		c.Items = append(c.Items, model.AdminCollectionItem{
			ItemID: m.ItemID, Kind: m.Kind, Title: m.Title, Year: optInt(m.Year), Position: int(m.Position),
		})
	}
	return c, nil
}

func toAdminCollection(r systemq.AdminGetCollectionRow) model.AdminCollection {
	c := model.AdminCollection{ID: r.ID, Source: r.Source, Name: r.Name, NameDe: r.NameDe.String, ItemCount: int(r.ItemCount)}
	if r.TmdbID.Valid {
		id := r.TmdbID.Int64
		c.TMDBID = &id
	}
	return c
}
