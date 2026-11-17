package wishes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/arr"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
)

// The profile tag in Radarr and Sonarr (concept 25 step 4.6b, architecture 20; N-20 to N-27,
// N-113). The job wishes.tag gives every requester of a wish «In Beschaffung» (owner and
// co-wishers) the tag of its profile at the film resp. series: it makes sure the tag exists
// (GET and POST /api/v3/tag), finds the title (movie?tmdbId, series?tvdbId) and attaches the tag
// with the editor and applyTags "add". A title not yet in Radarr or Sonarr stays pending, a refusing
// or silent service leaves the row failed; the next run sets it, the wish never changes (N-25).
// A tag is never removed (N-26). Wishes placed directly in Seerr get no tag (N-27).

// JobTag is the job type of the profile tag.
const JobTag = "wishes.tag"

const (
	// tagEvery is the schedule of wishes.tag beside the trigger after each sync (architecture 13).
	tagEvery          = 15 * time.Minute
	tagPriority       = 4
	tagManualPriority = jobs.ManualPriority
	maxTagLabel       = 64
	maxTagPrefix      = 10
)

// States of a tag row (wish_tags.state).
const (
	tagPending = "pending"
	tagSet     = "set"
	tagFailed  = "failed"
)

// Classes in wish_tags.last_error beside arrErrorClass.
const (
	tagErrNotInArr = "not_in_arr"      // the title is not yet in Radarr resp. Sonarr
	tagErrNoTVDB   = "tvdb_id_missing" // a series wish without TVDB id
)

// mediaShow is wishes.media of a series.
const mediaShow = "show"

// ErrTagNameTaken: another profile holds the tag name (409 tag_name_taken on the contract).
var ErrTagNameTaken = errors.New("tag name taken by another profile")

// registerArrTypes registers the job types that use Radarr and Sonarr.
func (s *Service) registerArrTypes(r *jobs.Registry) {
	r.Register(jobs.Type{
		Name: JobTag, Group: jobs.GroupSystem, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerSchedule,
		Priority: tagPriority, Handler: s.handleTags, Run: s.runTags, Every: tagEvery,
	})
}

func (s *Service) runTags(ctx context.Context, _ *jobs.Queue, prio int) ([]int64, error) {
	id, err := s.queueTags(ctx, prio)
	if err != nil {
		return nil, err
	}
	return []int64{id}, nil
}

// QueueTags queues one run of wishes.tag (or raises the waiting one): manual with «Profil-Tag
// setzen», otherwise with the priority of the schedule. The sync of Seerr calls it after each run
// and when a wish becomes processing or gets a co-wisher (architecture 13).
func (s *Service) QueueTags(ctx context.Context, manual bool) error {
	prio := tagPriority
	if manual {
		prio = tagManualPriority
	}
	_, err := s.queueTags(ctx, prio)
	return err
}

func (s *Service) queueTags(ctx context.Context, prio int) (int64, error) {
	if s.queue == nil {
		return 0, errors.New("wishes: no job queue")
	}
	id, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobTag, Priority: prio, IdempotencyKey: JobTag, Payload: []byte(`{}`)})
	if err != nil {
		return 0, fmt.Errorf("enqueue %s: %w", JobTag, err)
	}
	if _, err := s.queue.RaiseQueuedPriority(ctx, id, prio); err != nil {
		return 0, fmt.Errorf("raise %s: %w", JobTag, err)
	}
	return id, nil
}

// handleTags runs one pass. With the switch off it does nothing; errors of a service end in the
// rows and in the state of the service, never in the wish.
func (s *Service) handleTags(ctx context.Context, _ *jobs.Job) error {
	if on, err := s.boolState(ctx, SettingProfileTags, true); err != nil || !on {
		return err
	}
	return s.runTagPass(ctx)
}

func (s *Service) runTagPass(ctx context.Context) error {
	if err := s.addTagRows(ctx); err != nil {
		return err
	}
	rows, err := s.read.ListOpenWishTags(ctx)
	if err != nil {
		return fmt.Errorf("list open wish tags: %w", err)
	}
	byService := map[arr.Kind][]wishq.ListOpenWishTagsRow{}
	for _, r := range rows {
		byService[arr.Kind(r.Service)] = append(byService[arr.Kind(r.Service)], r)
	}
	for _, kind := range []arr.Kind{arr.Radarr, arr.Sonarr} {
		if len(byService[kind]) > 0 {
			if err := s.tagService(ctx, kind, byService[kind]); err != nil {
				return err
			}
		}
	}
	return nil
}

// addTagRows writes a pending row for every requester of a wish in procurement that has none.
func (s *Service) addTagRows(ctx context.Context) error {
	cands, err := s.read.ListTagCandidates(ctx)
	if err != nil {
		return fmt.Errorf("list tag candidates: %w", err)
	}
	for _, c := range cands {
		name, err := s.profileTagName(ctx, c.ProfileID)
		if err != nil {
			return err
		}
		service := string(arr.Radarr)
		if c.Media == mediaShow {
			service = string(arr.Sonarr)
		}
		if _, err := s.write.InsertWishTag(ctx, wishq.InsertWishTagParams{WishID: c.WishID, ProfileID: c.ProfileID, Service: service, TagName: name}); err != nil {
			return fmt.Errorf("insert wish tag: %w", err)
		}
	}
	return nil
}

// tagService sets the open rows of one service. A service that cannot be reached marks the rows
// failed; an unconfirmed address halts without touching them.
func (s *Service) tagService(ctx context.Context, kind arr.Kind, rows []wishq.ListOpenWishTagsRow) error {
	c, _, err := s.arrClient(ctx, kind, true)
	if err != nil {
		s.recordArr(ctx, kind, err)
		if errors.Is(err, ErrArrHostUnconfirmed) {
			s.log.WarnContext(ctx, "profile tags halted: address of the service not confirmed", "service", string(kind))
		}
		return nil
	}
	tagIDs := map[string]int{}
	titles := map[int64]int{} // wish id -> title id of this run
	var lastErr error
	for i, r := range rows {
		err := s.tagOne(ctx, c, kind, r, tagIDs, titles)
		if err == nil {
			continue
		}
		s.arrFailed(kind, err)
		if !serviceDown(err) {
			lastErr = err
			if e := s.openTag(ctx, r, tagFailed, arrErrorClass(err), 0, 0); e != nil {
				return e
			}
			continue
		}
		// The service is gone: the rest of the run would fail the same way.
		for _, rest := range rows[i:] {
			if e := s.openTag(ctx, rest, tagFailed, arrErrorClass(err), 0, 0); e != nil {
				return e
			}
		}
		s.recordArr(ctx, kind, err)
		s.log.WarnContext(ctx, "profile tags not set", "service", string(kind), "err", arrErrorClass(err))
		return nil
	}
	s.recordArr(ctx, kind, lastErr)
	return nil
}

// tagOne sets the tag of one row; nil also when the title is not yet there (the row stays pending).
func (s *Service) tagOne(ctx context.Context, c *arr.Client, kind arr.Kind, r wishq.ListOpenWishTagsRow,
	tagIDs map[string]int, titles map[int64]int,
) error {
	tagID, ok := tagIDs[r.TagName]
	if !ok {
		tag, created, err := c.EnsureTag(ctx, r.TagName)
		if err != nil {
			return err
		}
		if created {
			s.log.InfoContext(ctx, "profile tag created", "service", string(kind), "tag", r.TagName)
		}
		tagID, tagIDs[r.TagName] = tag.ID, tag.ID
	}
	titleID, known := titles[r.WishID]
	if !known && r.ArrTitleID.Valid {
		titleID, known = int(r.ArrTitleID.Int64), true
	}
	var tags []int
	if !known {
		var err error
		titleID, tags, err = s.findTitle(ctx, c, kind, r.TmdbID, r.TvdbID)
		switch {
		case errors.Is(err, errNoTVDB):
			return s.openTag(ctx, r, tagPending, tagErrNoTVDB, tagID, 0)
		case errors.Is(err, arr.ErrNotFound):
			return s.openTag(ctx, r, tagPending, tagErrNotInArr, tagID, 0)
		case err != nil:
			return err
		}
		titles[r.WishID] = titleID
	}
	if !containsInt(tags, tagID) {
		if err := c.AttachTag(ctx, tagID, titleID); err != nil {
			return err
		}
	}
	if err := s.write.SetWishTagDone(ctx, wishq.SetWishTagDoneParams{
		ArrTagID: sql.NullInt64{Int64: int64(tagID), Valid: true}, ArrTitleID: sql.NullInt64{Int64: int64(titleID), Valid: true},
		Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, WishID: r.WishID, ProfileID: r.ProfileID,
	}); err != nil {
		return fmt.Errorf("store wish tag: %w", err)
	}
	return nil
}

var errNoTVDB = errors.New("series wish without tvdb id")

// findTitle finds the film by TMDB id resp. the series by TVDB id; arr.ErrNotFound when the
// service does not have it yet.
func (s *Service) findTitle(ctx context.Context, c *arr.Client, kind arr.Kind, tmdbID int64, tvdbID sql.NullInt64) (int, []int, error) {
	if kind == arr.Radarr {
		m, err := c.MovieByTMDB(ctx, int(tmdbID))
		return m.ID, m.Tags, err
	}
	if !tvdbID.Valid {
		return 0, nil, errNoTVDB
	}
	se, err := c.SeriesByTVDB(ctx, int(tvdbID.Int64))
	return se.ID, se.Tags, err
}

func (s *Service) openTag(ctx context.Context, r wishq.ListOpenWishTagsRow, state, class string, tagID, titleID int) error {
	p := wishq.SetWishTagOpenParams{State: state, LastError: nullString(class), WishID: r.WishID, ProfileID: r.ProfileID}
	if tagID > 0 {
		p.ArrTagID = sql.NullInt64{Int64: int64(tagID), Valid: true}
	}
	if titleID > 0 {
		p.ArrTitleID = sql.NullInt64{Int64: int64(titleID), Valid: true}
	}
	if err := s.write.SetWishTagOpen(ctx, p); err != nil {
		return fmt.Errorf("store wish tag: %w", err)
	}
	return nil
}

func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// tagPrefix returns the prefix setting; an invalid one is the default.
func (s *Service) tagPrefix(ctx context.Context) (string, error) {
	v, err := s.state(ctx, SettingTagPrefix)
	if err != nil {
		return "", err
	}
	if !ValidTagPrefix(v) {
		return DefaultTagPrefix, nil
	}
	return v, nil
}

// ValidTagPrefix checks a prefix: ^[a-z0-9-]{1,10}$ (data model 2.12).
func ValidTagPrefix(v string) bool {
	if v == "" || len(v) > maxTagPrefix {
		return false
	}
	return strings.Trim(v, "abcdefghijklmnopqrstuvwxyz0123456789-") == ""
}

// profileTagName returns the tag name of a profile and stores it at the first call: a name in
// wish_profile_tags stays as it is (N-21). Otherwise the name is prefix and profile name after
// N-113 (arr.ProfileLabel), <prefix>profil-<id> when the name gives no letter or digit. When an
// earlier profile (smaller id) without a stored name gives the same name, or another profile holds
// it, the account's name is added (K-18), and when that is taken as well the fallback is used.
func (s *Service) profileTagName(ctx context.Context, profileID int64) (string, error) {
	stored, err := s.read.GetProfileTag(ctx, profileID)
	if err == nil {
		return stored.Name, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("read profile tag: %w", err)
	}
	prefix, err := s.tagPrefix(ctx)
	if err != nil {
		return "", err
	}
	p, err := s.read.GetTagProfile(ctx, profileID)
	if err != nil {
		return "", fmt.Errorf("read profile %d: %w", profileID, err)
	}
	earlier, err := s.read.ListEarlierProfilesWithoutTag(ctx, profileID)
	if err != nil {
		return "", fmt.Errorf("list profiles: %w", err)
	}
	taken := func(label string) (bool, error) {
		owner, err := s.read.TagNameOwner(ctx, label)
		switch {
		case err == nil:
			return owner != profileID, nil
		case !errors.Is(err, sql.ErrNoRows):
			return false, fmt.Errorf("read tag name: %w", err)
		}
		for _, e := range earlier {
			if l, ok := arr.ProfileLabel(prefix, e.Name); ok && clampLabel(l) == label {
				return true, nil
			}
		}
		return false, nil
	}
	fallback := clampLabel(arr.Slug(prefix + "profil-" + strconv.FormatInt(profileID, 10)))
	label := fallback
	if l, ok := arr.ProfileLabel(prefix, p.Name); ok {
		label = clampLabel(l)
	}
	if label != fallback {
		busy, err := taken(label)
		if err != nil {
			return "", err
		}
		if busy {
			label = fallback
			if acc := arr.Slug(p.AccountName); acc != "" {
				cand := clampLabel(arr.Slug(label0(prefix, p.Name) + "-" + acc))
				if busy, err := taken(cand); err != nil {
					return "", err
				} else if !busy && cand != "" {
					label = cand
				}
			}
		}
	}
	n, err := s.write.InsertProfileTag(ctx, wishq.InsertProfileTagParams{ProfileID: profileID, Name: label, Now: s.now().UnixMilli()})
	if err != nil {
		return "", fmt.Errorf("store profile tag: %w", err)
	}
	if n == 0 { // stored meanwhile
		stored, err := s.read.GetProfileTag(ctx, profileID)
		if err != nil {
			return "", fmt.Errorf("read profile tag: %w", err)
		}
		return stored.Name, nil
	}
	return label, nil
}

// label0 is the plain label of a profile name (the name has a letter or digit).
func label0(prefix, name string) string {
	l, _ := arr.ProfileLabel(prefix, name)
	return l
}

// clampLabel keeps a label within maxTagLabel characters without a dash at the end.
func clampLabel(l string) string {
	if len(l) > maxTagLabel {
		l = strings.TrimRight(l[:maxTagLabel], "-")
	}
	return l
}

// SetProfileTagName sets the tag name of a profile (setProfileTagName); nil or "" goes back to the
// name from prefix and profile name, stored again at the next tag. The name is checked after
// N-113 (model.ValidationError on name, 422), must be free (ErrTagNameTaken, 409) and the profile
// must exist (model.ErrNotFound, 404). It names the tags set from now on; tags already set stay
// on their titles (N-26).
func (s *Service) SetProfileTagName(ctx context.Context, profileID int64, name *string) error {
	n, err := s.read.CountProfiles(ctx, profileID)
	if err != nil {
		return fmt.Errorf("read profile: %w", err)
	}
	if n == 0 {
		return model.ErrNotFound
	}
	if name == nil || strings.TrimSpace(*name) == "" {
		if err := s.write.DeleteProfileTag(ctx, profileID); err != nil {
			return fmt.Errorf("delete profile tag: %w", err)
		}
		return nil
	}
	v := strings.TrimSpace(*name)
	if len(v) > maxTagLabel || arr.ValidLabel(v) != nil {
		return (&model.ValidationError{}).Add("name", "invalid_value")
	}
	owner, err := s.read.TagNameOwner(ctx, v)
	switch {
	case err == nil && owner != profileID:
		return ErrTagNameTaken
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("read tag name: %w", err)
	}
	if err := s.write.SetCustomProfileTag(ctx, wishq.SetCustomProfileTagParams{ProfileID: profileID, Name: v, Now: s.now().UnixMilli()}); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return ErrTagNameTaken
		}
		return fmt.Errorf("store profile tag: %w", err)
	}
	return nil
}

// ProfileTag is the tag name of a profile for the administration (WishMappings.profiles[]).
type ProfileTag struct {
	Name   string
	Custom bool // set by the administrator
}

// ProfileTagOf returns the tag name a profile has or would get now. It stores nothing for a
// profile without a stored name (the name is stored at its first tag).
func (s *Service) ProfileTagOf(ctx context.Context, profileID int64) (ProfileTag, error) {
	stored, err := s.read.GetProfileTag(ctx, profileID)
	if err == nil {
		return ProfileTag{Name: stored.Name, Custom: stored.Custom == 1}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ProfileTag{}, fmt.Errorf("read profile tag: %w", err)
	}
	prefix, err := s.tagPrefix(ctx)
	if err != nil {
		return ProfileTag{}, err
	}
	p, err := s.read.GetTagProfile(ctx, profileID)
	if errors.Is(err, sql.ErrNoRows) {
		return ProfileTag{}, model.ErrNotFound
	}
	if err != nil {
		return ProfileTag{}, fmt.Errorf("read profile: %w", err)
	}
	if l, ok := arr.ProfileLabel(prefix, p.Name); ok {
		return ProfileTag{Name: clampLabel(l)}, nil
	}
	return ProfileTag{Name: clampLabel(arr.Slug(prefix + "profil-" + strconv.FormatInt(profileID, 10)))}, nil
}

// WishTag is the tag state of one requester of a wish (AdminWish.requesters[].tag).
type WishTag struct {
	ProfileID int64
	Service   string // radarr or sonarr
	Name      string
	State     string // pending, set, failed
	LastError string // error class, never an address or a key
	SetAt     time.Time
}

// WishTagsOf returns the tag rows of the wishes by wish id.
func (s *Service) WishTagsOf(ctx context.Context, wishIDs []int64) (map[int64][]WishTag, error) {
	out := map[int64][]WishTag{}
	if len(wishIDs) == 0 {
		return out, nil
	}
	rows, err := s.read.ListWishTagsOf(ctx, wishIDs)
	if err != nil {
		return nil, fmt.Errorf("list wish tags: %w", err)
	}
	for _, r := range rows {
		t := WishTag{ProfileID: r.ProfileID, Service: r.Service, Name: r.TagName, State: r.State, LastError: r.LastError.String}
		if r.SetAt.Valid {
			t.SetAt = time.UnixMilli(r.SetAt.Int64)
		}
		out[r.WishID] = append(out[r.WishID], t)
	}
	return out, nil
}
