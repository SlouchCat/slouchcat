package visibility

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// M5: the profiles of an account that may see a title (downloads.allowed_profile_ids, plan 3.3),
// read for every title against the fixture oracle, System scope only.
func TestAllowedProfiles(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddEdgeCases(t)
	f.AddSeries(t)
	f.AddGuest(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	all := f.All()
	ids := make([]string, 0, len(all)+1)
	for _, m := range all {
		ids = append(ids, m.ID)
	}
	ids = append(ids, "01J0000000000000000000NONE", all[0].ID) // unknown and duplicate ids
	for _, acc := range []struct {
		id       int64
		profiles []model.Profile
	}{
		{f.Account.ID, []model.Profile{f.P18, f.P16, f.P12, f.P6, f.P0, f.PX}},
		{f.Guest.ID, []model.Profile{f.PG, f.PG18}},
	} {
		got, err := r.AllowedProfiles(ctx, System("test"), acc.id, ids)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range all {
			var want []int64
			for _, p := range acc.profiles {
				if f.VisibleTo(p, m) {
					want = append(want, p.ID)
				}
			}
			slices.Sort(want)
			if !slices.Equal(got[m.ID], want) {
				t.Errorf("account %d, %s %s: allowed %v, want %v", acc.id, m.Kind, m.Label, got[m.ID], want)
			}
		}
		if _, ok := got["01J0000000000000000000NONE"]; ok {
			t.Error("unknown title has allowed profiles")
		}
	}
	// The guest's profiles never appear for the owner's titles of library X and vice versa.
	got, _ := r.AllowedProfiles(ctx, System("test"), f.Guest.ID, []string{f.Rated(16).ID})
	if len(got) != 0 {
		t.Errorf("guest sees the 16 film: %v", got)
	}
	if _, err := r.AllowedProfiles(ctx, Profile(f.P18), f.Account.ID, ids); !errors.Is(err, ErrNotSystem) {
		t.Errorf("profile scope: %v, want ErrNotSystem", err)
	}
	if got, err := r.AllowedProfiles(ctx, System("test"), f.Account.ID, nil); err != nil || len(got) != 0 {
		t.Errorf("no ids: %v, %v", got, err)
	}
}

func TestSummariesAndEpisodes(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	ids := []string{f.Rated(6).ID, f.Rated(12).ID, f.Show.E2.ID, "01J0000000000000000000NONE"}
	got, err := r.Summaries(ctx, Profile(f.P12), ids)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[f.Rated(12).ID].Title == "" || got[f.Rated(6).ID].Poster == nil {
		t.Errorf("P12 summaries = %v", got)
	}
	if _, ok := got[f.Show.E2.ID]; ok {
		t.Error("P12 got the episode rated 16")
	}
	if _, err := r.Summaries(ctx, System("test"), ids); !errors.Is(err, ErrUnscoped) {
		t.Errorf("system scope: %v", err)
	}

	eps, err := r.Episodes(ctx, Profile(f.P12), f.Show.Season.ID)
	if err != nil {
		t.Fatal(err)
	}
	var list []string
	for _, ep := range eps {
		list = append(list, ep.ID)
	}
	if want := []string{f.Show.E1.ID, f.Show.E3.ID}; !slices.Equal(list, want) {
		t.Errorf("P12 episodes = %v, want %v", list, want)
	}
	if eps, _ := r.Episodes(ctx, Profile(f.P16), f.Show.Season.ID); len(eps) != 3 {
		t.Errorf("P16 episodes = %d, want 3", len(eps))
	}
	for _, tc := range []struct {
		p  model.Profile
		id string
	}{{f.P6, f.Show.Season.ID}, {f.P18, f.Show.Series.ID}, {f.P18, f.Rated(6).ID}} {
		if _, err := r.Episodes(ctx, Profile(tc.p), tc.id); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s episodes of %s: %v, want ErrNotFound", tc.p.Name, tc.id, err)
		}
	}
}
