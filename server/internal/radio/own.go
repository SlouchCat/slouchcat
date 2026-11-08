package radio

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/oklog/ulid/v2"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/radio/rules"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Own stations (concept 16 section 5.6, F-15, plan D-8, contract 1.19.0): stations the catalog
// lacks, which the administration adds, changes and deletes. Their stream address and homepage
// must be http or https and resolve to public addresses only: the host is looked up (resolver of
// the service) and every address checked with netguard.Local, and the entries of a .pls or .m3u
// address are checked the same way at create. A local address is the field code local_address,
// also for own stations (F-15); playing stays guarded by the dialer. The facets given are manual
// and locked like a PATCH, the others derive from the name. A probe after the create (and after
// a new address) fills codec, bitrate and quality, best effort. The sync never touches an own
// station: it has no alias and its group key is own:<ULID>.

const (
	// sourceOwn and sourceCatalog are the values of radio_stations.source.
	sourceOwn     = "own"
	sourceCatalog = "catalog"
	// ownURLMax is the contract's maxLength of streamUrl and homepage.
	ownURLMax = 2048
	// ownResolveTimeout bounds the lookup of one host.
	ownResolveTimeout = 5 * time.Second
	// ownPlaylistTimeout bounds the fetch of a .pls or .m3u at create.
	ownPlaylistTimeout = 10 * time.Second
	// ownPlaylistEntries bounds the entries of a playlist that are checked.
	ownPlaylistEntries = 20
)

// Field codes of the own stations (contract createRadioStation, updateAdminRadioStation).
const (
	codeInvalidValue = "invalid_value"
	codeLocalAddress = "local_address"
	codeNotOwn       = "not_own"
)

// languageCode is an ISO 639-1 code.
var languageCode = regexp.MustCompile(`^[a-z]{2}$`)

// ownProbeFunc reads codec and bitrate of a stream and whether it is HLS.
type ownProbeFunc func(ctx context.Context, rawURL string) (transcode.RadioInput, bool, error)

// lookupHost returns the addresses of host through the resolver of the catalog part (tests set
// one), else the system resolver.
func (s *Service) lookupHost(ctx context.Context, host string) ([]netip.Addr, error) {
	if s.cat.resolve != nil {
		return s.cat.resolve(ctx, host)
	}
	return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
}

// publicURLCode checks an address of an own station: "" when it is http or https and its host
// is or resolves to public addresses only, local_address when one is local, invalid_value for
// anything else (an unparsable address, another scheme, an unknown host).
func (s *Service) publicURLCode(ctx context.Context, raw string) string {
	if raw == "" || len(raw) > ownURLMax {
		return codeInvalidValue
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return codeInvalidValue
	}
	host := u.Hostname()
	if ip, err := netip.ParseAddr(host); err == nil {
		if netguard.Local(ip) {
			return codeLocalAddress
		}
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, ownResolveTimeout)
	defer cancel()
	addrs, err := s.lookupHost(ctx, host)
	if err != nil || len(addrs) == 0 {
		return codeInvalidValue
	}
	if slices.ContainsFunc(addrs, netguard.Local) {
		return codeLocalAddress
	}
	return ""
}

// streamURLCode is publicURLCode for a stream address; a .pls or .m3u is fetched and each of its
// entries checked as well (a local entry is local_address). A playlist that cannot be fetched
// now passes: the dialer still guards it when it plays.
func (s *Service) streamURLCode(ctx context.Context, raw string) string {
	if code := s.publicURLCode(ctx, raw); code != "" {
		return code
	}
	for _, entry := range s.playlistEntries(ctx, raw) {
		if s.publicURLCode(ctx, entry) == codeLocalAddress {
			return codeLocalAddress
		}
	}
	return ""
}

// playlistEntries returns the http and https entries of a .pls or .m3u address (nil for any other
// address or a failed fetch), through the guarded client of the catalog part.
func (s *Service) playlistEntries(ctx context.Context, raw string) []string {
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	if ext := strings.ToLower(path.Ext(u.Path)); ext != ".pls" && ext != ".m3u" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, ownPlaylistTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", s.cat.fetch.userAgent)
	resp, err := s.cat.fetch.httpClient().Do(req)
	if err != nil {
		s.log.InfoContext(ctx, "radio own playlist not fetched", "site", netguard.Site(raw), "err", netguard.Clean(err))
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPlaylistBytes))
	if err != nil && len(body) == 0 {
		return nil
	}
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() && len(out) < ownPlaylistEntries {
		line := strings.TrimSpace(sc.Text())
		if k, v, ok := strings.Cut(line, "="); ok && strings.HasPrefix(strings.ToLower(k), "file") {
			line = strings.TrimSpace(v) // .pls: FileN=<url>
		}
		if l := strings.ToLower(line); strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") {
			out = append(out, line)
		}
	}
	return out
}

// ownLanguages checks and orders a language list (ISO 639-1, no repeats).
func ownLanguages(list []string) ([]string, bool) {
	out := []string{}
	for _, l := range list {
		if !languageCode.MatchString(l) {
			return nil, false
		}
		if !slices.Contains(out, l) {
			out = append(out, l)
		}
	}
	return out, true
}

// CreateStation adds an own station (contract createRadioStation).
func (s *Service) CreateStation(ctx context.Context, c model.AdminRadioStationCreate) (model.AdminRadioStation, error) {
	var verr *model.ValidationError
	name := strings.TrimSpace(c.Name)
	if n := utf8.RuneCountInString(name); n < 1 || n > syncNameMax {
		verr = verr.Add("name", codeInvalidValue)
	}
	streamURL := strings.TrimSpace(c.StreamURL)
	if code := s.streamURLCode(ctx, streamURL); code != "" {
		verr = verr.Add("streamUrl", code)
	}
	homepage := strings.TrimSpace(c.Homepage)
	if homepage != "" {
		if code := s.publicURLCode(ctx, homepage); code != "" {
			verr = verr.Add("homepage", code)
		}
	}
	if c.Country != "" && !countryCode.MatchString(c.Country) {
		verr = verr.Add("country", codeInvalidValue)
	}
	if c.Region != "" && !isoRegion.MatchString(c.Region) {
		verr = verr.Add("region", codeInvalidValue)
	}
	langs, ok := ownLanguages(c.Languages)
	if !ok {
		verr = verr.Add("languages", codeInvalidValue)
	}
	given := map[string][]string{rules.FacetGenre: c.Genres, rules.FacetKind: c.Kinds, rules.FacetMood: c.Moods}
	var manual, open []string
	for _, facet := range facetNames {
		if len(given[facet]) == 0 {
			open = append(open, facet)
			continue
		}
		manual = append(manual, facet)
		for _, v := range given[facet] {
			if !knownFacetValue(facet, v) {
				verr = verr.Add(facet+"s", codeInvalidValue)
				break
			}
		}
	}
	if verr != nil {
		return model.AdminRadioStation{}, verr
	}
	rs, err := s.tagRules(ctx)
	if err != nil {
		return model.AdminRadioStation{}, err
	}
	derived, _ := deriveFacets(rs.tags, name, nil)

	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return model.AdminRadioStation{}, fmt.Errorf("create radio station: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := appq.New(tx)
	now := s.now().UnixMilli()
	id, err := q.InsertRadioOwnStation(ctx, appq.InsertRadioOwnStationParams{GroupKey: "own:" + ulid.Make().String(),
		Name: name, SortKey: rules.Fold(name), SearchText: searchText(name, c.Region, nil), Country: nullText(c.Country),
		Region: nullText(c.Region), Languages: jsonList(langs), Homepage: nullText(homepage), Now: now})
	if err != nil {
		return model.AdminRadioStation{}, fmt.Errorf("insert radio station: %w", err)
	}
	if err := q.InsertRadioOwnStream(ctx, appq.InsertRadioOwnStreamParams{StationID: id, Url: streamURL, Now: now}); err != nil {
		return model.AdminRadioStation{}, fmt.Errorf("insert radio stream: %w", err)
	}
	if err := writeFacets(ctx, q, id, derived, open, false, false); err != nil {
		return model.AdminRadioStation{}, err
	}
	if err := writeFacets(ctx, q, id, given, manual, true, false); err != nil {
		return model.AdminRadioStation{}, err
	}
	locks := manual
	if c.Region != "" {
		locks = append(locks, "region")
	}
	for _, l := range locks {
		if err := q.InsertRadioStationLock(ctx, appq.InsertRadioStationLockParams{StationID: id, Field: l}); err != nil {
			return model.AdminRadioStation{}, fmt.Errorf("insert radio lock: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return model.AdminRadioStation{}, fmt.Errorf("create radio station: commit: %w", err)
	}
	s.log.InfoContext(ctx, "radio own station created", "station_id", id, "site", netguard.Site(streamURL))
	s.reloadStation(ctx, id)
	s.queueLogos(ctx, []int64{id})
	s.probeOwnLater(id, streamURL)
	return s.AdminStation(ctx, id)
}

// ownPatch is the checked own-station part of a PATCH (UpdateStation).
type ownPatch struct {
	set       bool // the patch names an own-station field
	streamURL string
	homepage  sql.NullString
	country   sql.NullString
	languages string
}

// checkOwnPatch checks the fields of a PATCH only an own station has (streamUrl, homepage,
// country, languages): on a catalog station each is not_own. The lookups run before the write
// transaction, which never waits for the network.
func (s *Service) checkOwnPatch(ctx context.Context, stationID int64, p model.AdminRadioStationPatch) (ownPatch, *model.ValidationError, error) {
	fields := map[string]bool{"streamUrl": p.StreamURL != nil, "homepage": p.Homepage != nil || p.ClearHomepage,
		"country": p.Country != nil || p.ClearCountry, "languages": p.Languages != nil}
	if !fields["streamUrl"] && !fields["homepage"] && !fields["country"] && !fields["languages"] {
		return ownPatch{}, nil, nil
	}
	row, err := appq.New(s.db.Read).GetRadioStation(ctx, stationID)
	if errors.Is(err, sql.ErrNoRows) {
		return ownPatch{}, nil, model.ErrNotFound
	}
	if err != nil {
		return ownPatch{}, nil, fmt.Errorf("get radio station: %w", err)
	}
	var verr *model.ValidationError
	if row.Source != sourceOwn {
		for _, f := range []string{"streamUrl", "homepage", "country", "languages"} {
			if fields[f] {
				verr = verr.Add(f, codeNotOwn)
			}
		}
		return ownPatch{}, verr, nil
	}
	op := ownPatch{set: true, homepage: row.Homepage, country: row.Country, languages: row.Languages}
	if p.StreamURL != nil {
		op.streamURL = strings.TrimSpace(*p.StreamURL)
		if code := s.streamURLCode(ctx, op.streamURL); code != "" {
			verr = verr.Add("streamUrl", code)
		}
	}
	switch {
	case p.ClearHomepage:
		op.homepage = sql.NullString{}
	case p.Homepage != nil:
		op.homepage = nullText(strings.TrimSpace(*p.Homepage))
		if op.homepage.Valid {
			if code := s.publicURLCode(ctx, op.homepage.String); code != "" {
				verr = verr.Add("homepage", code)
			}
		}
	}
	switch {
	case p.ClearCountry:
		op.country = sql.NullString{}
	case p.Country != nil:
		if !countryCode.MatchString(*p.Country) {
			verr = verr.Add("country", codeInvalidValue)
		}
		op.country = nullText(*p.Country)
	}
	if p.Languages != nil {
		langs, ok := ownLanguages(*p.Languages)
		if !ok {
			verr = verr.Add("languages", codeInvalidValue)
		}
		op.languages = jsonList(langs)
	}
	return op, verr, nil
}

// writeOwnPatch writes the checked own-station fields in the transaction of UpdateStation and
// reports whether the stream address changed.
func writeOwnPatch(ctx context.Context, q *appq.Queries, stationID int64, op ownPatch, now int64) (bool, error) {
	if !op.set {
		return false, nil
	}
	if err := q.UpdateRadioOwnStation(ctx, appq.UpdateRadioOwnStationParams{Homepage: op.homepage, Country: op.country,
		Languages: op.languages, Now: now, ID: stationID}); err != nil {
		return false, fmt.Errorf("update own radio station: %w", err)
	}
	if op.streamURL == "" {
		return false, nil
	}
	streams, err := q.ListRadioStreamsRanked(ctx, stationID)
	if err != nil {
		return false, fmt.Errorf("list radio streams: %w", err)
	}
	if len(streams) > 0 && streams[0].Url == op.streamURL {
		return false, nil
	}
	if err := q.UpdateRadioOwnStreamURL(ctx, appq.UpdateRadioOwnStreamURLParams{Url: op.streamURL, Now: now, StationID: stationID}); err != nil {
		return false, fmt.Errorf("update own radio stream: %w", err)
	}
	return true, nil
}

// DeleteStation deletes an own station (contract deleteRadioStation): the row goes with its
// streams, facets, locks, favourites and listen sessions (cascade), the index drops it, and only
// then its relay and open listen sessions end, so a Listen in between finds no station to start
// a relay for (404); its logo files go.
func (s *Service) DeleteStation(ctx context.Context, stationID int64) error {
	row, err := appq.New(s.db.Read).GetRadioStation(ctx, stationID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get radio station: %w", err)
	}
	if row.Source != sourceOwn {
		return (*model.ValidationError)(nil).Add("stationId", codeNotOwn)
	}
	n, err := appq.New(s.db.Write).DeleteRadioOwnStation(ctx, stationID)
	if err != nil {
		return fmt.Errorf("delete radio station: %w", err)
	}
	if n == 0 {
		return model.ErrNotFound
	}
	s.reloadStation(ctx, stationID)
	// The rows of the open listen sessions went with the station, so closeStation ends none in
	// memory: their ids are taken here.
	var open []string
	s.rel.mu.Lock()
	for id, ls := range s.rel.sessions {
		if ls.stationID == stationID {
			open = append(open, id)
		}
	}
	s.rel.mu.Unlock()
	s.closeStation(ctx, stationID)
	s.endLocal(ctx, open)
	if s.o.cacheDir != "" {
		if err := os.RemoveAll(filepath.Join(s.o.cacheDir, logoDir(stationID))); err != nil {
			s.log.WarnContext(ctx, "remove radio logo files", "station_id", stationID, "err", err)
		}
	}
	s.log.InfoContext(ctx, "radio own station deleted", "station_id", stationID)
	return nil
}

// probeOwnLater probes the stream of an own station in the background of the catalog part
// (probeTimeout, ended by the shutdown) and stores codec, bitrate and quality while the stream
// still has that address; a failed probe changes nothing (the create or change stands).
func (s *Service) probeOwnLater(stationID int64, rawURL string) {
	s.cat.bg.goFn(func(ctx context.Context) {
		ctx, cancel := context.WithTimeout(ctx, probeTimeout)
		defer cancel()
		probe := s.cat.probe
		if probe == nil {
			probe = s.probeStream
		}
		in, hls, err := probe(ctx, rawURL)
		if err != nil {
			s.log.InfoContext(ctx, "radio own station probe failed", "station_id", stationID, "err", netguard.Clean(err))
			return
		}
		q := appq.New(s.db.Write)
		now := s.now().UnixMilli()
		codec := strings.ToLower(in.Codec)
		n, err := q.SetRadioOwnStreamProbe(ctx, appq.SetRadioOwnStreamProbeParams{Codec: nullText(codec),
			Bitrate: sql.NullInt64{Int64: int64(in.BitrateKbps), Valid: in.BitrateKbps > 0}, Hls: flag01(hls), Now: now,
			StationID: stationID, Url: rawURL})
		if err != nil || n == 0 {
			if err != nil {
				s.log.WarnContext(ctx, "store radio own station probe", "station_id", stationID, "err", err)
			}
			return
		}
		if err := q.SetRadioStationQuality(ctx, appq.SetRadioStationQualityParams{
			Quality: nullText(qualityOf(codec, int64(in.BitrateKbps))), Now: now, ID: stationID}); err != nil {
			s.log.WarnContext(ctx, "store radio own station quality", "station_id", stationID, "err", err)
			return
		}
		s.reloadStation(ctx, stationID)
	})
}

// probeStream opens a stream through the guarded client and reports its input and HLS flag.
func (s *Service) probeStream(ctx context.Context, rawURL string) (transcode.RadioInput, bool, error) {
	st, err := s.netClient().openStream(ctx, rawURL)
	if err != nil {
		return transcode.RadioInput{}, false, err
	}
	defer func() { _ = st.Close() }()
	info := st.Info()
	in := info.Input
	if in.BitrateKbps == 0 {
		in.BitrateKbps = info.ICYBitrate
	}
	return in, info.HLS, nil
}
