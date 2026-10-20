package segments

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"
)

// tolerance is the accuracy W-9 asks for.
const tolerance = 2 * time.Second

// fingerprints builds synthetic fingerprints: random values (unrelated audio) with shared blocks
// copied in, each copy with a few flipped bits like a second encode of the same audio.
type fingerprints struct{ rng *rand.Rand }

func newFingerprints(seed uint64) *fingerprints {
	return &fingerprints{rng: rand.New(rand.NewPCG(seed, seed*7+1))}
}

func (f *fingerprints) noise(d time.Duration) []uint32 {
	v := make([]uint32, int(d/Step))
	for i := range v {
		v[i] = f.rng.Uint32()
	}
	return v
}

// put copies block into fp at time at, flipping up to 3 bits per value.
func (f *fingerprints) put(fp, block []uint32, at time.Duration) {
	p := int(at / Step)
	for i, v := range block {
		for range f.rng.IntN(4) {
			v ^= 1 << f.rng.IntN(32)
		}
		fp[p+i] = v
	}
}

// season describes synthetic episodes: the intro start per episode (negative: no intro) and
// whether the outro block ends the file.
type season struct {
	duration   time.Duration
	introAt    []time.Duration
	introLen   time.Duration
	outroLen   time.Duration
	outroAhead time.Duration // gap between the end of the outro block and the end of the file
}

func (s season) build(t *testing.T, seed uint64) []Episode {
	t.Helper()
	f := newFingerprints(seed)
	o := DefaultOptions()
	intro := f.noise(s.introLen)
	outro := f.noise(s.outroLen)
	var eps []Episode
	for i, at := range s.introAt {
		e := Episode{ID: fmt.Sprintf("e%d", i+1), Duration: s.duration + time.Duration(i)*7*time.Second}
		e.Head = f.noise(o.HeadLength(e.Duration))
		if at >= 0 {
			f.put(e.Head, intro, at)
		}
		from, length := o.TailRange(e.Duration)
		e.Tail = f.noise(length)
		if s.outroLen > 0 {
			f.put(e.Tail, outro, e.Duration-s.outroAhead-s.outroLen-from)
		}
		eps = append(eps, e)
	}
	return eps
}

func near(got *Span, start, end time.Duration) bool {
	return got != nil && absDur(got.Start-start) <= tolerance && absDur(got.End-end) <= tolerance
}

func TestDetectSyntheticSeason(t *testing.T) {
	s := season{
		duration: 22 * time.Minute, introLen: 20 * time.Second, outroLen: 40 * time.Second,
		introAt: []time.Duration{30 * time.Second, 45 * time.Second, 62 * time.Second, 90 * time.Second, 2*time.Minute + 3*time.Second},
	}
	eps := s.build(t, 1)
	res := Detect(eps, DefaultOptions())
	introOK, outroOK := 0, 0
	for i, e := range eps {
		r := res[e.ID]
		t.Logf("%s: intro %+v (want %v), outro %+v (want %v-%v)", e.ID, r.Intro, s.introAt[i], r.Outro, e.Duration-s.outroLen, e.Duration)
		if near(r.Intro, s.introAt[i], s.introAt[i]+s.introLen) && r.Intro.Source == SourceFingerprint && r.Intro.Confidence > 0.5 {
			introOK++
		}
		if near(r.Outro, e.Duration-s.outroLen, e.Duration) && r.Outro.Source == SourceFingerprint {
			outroOK++
		}
	}
	if introOK*5 < 4*len(eps) || outroOK*5 < 4*len(eps) {
		t.Errorf("intro within ±2 s for %d, outro for %d of %d episodes; want >= 80 %%", introOK, outroOK, len(eps))
	}
}

func TestDetectNoSharedBlock(t *testing.T) {
	f := newFingerprints(2)
	o := DefaultOptions()
	var eps []Episode
	for i := range 5 {
		d := 20 * time.Minute
		_, tl := o.TailRange(d)
		eps = append(eps, Episode{ID: fmt.Sprint(i), Duration: d, Head: f.noise(o.HeadLength(d)), Tail: f.noise(tl)})
	}
	if res := Detect(eps, o); len(res) != 0 {
		t.Errorf("Detect found %+v in unrelated episodes", res)
	}
}

func TestDetectTwoEpisodes(t *testing.T) {
	s := season{duration: 3 * time.Minute, introLen: 25 * time.Second, outroLen: 30 * time.Second,
		introAt: []time.Duration{10 * time.Second, 40 * time.Second}}
	eps := s.build(t, 3)
	res := Detect(eps, DefaultOptions())
	for i, e := range eps {
		r := res[e.ID]
		if !near(r.Intro, s.introAt[i], s.introAt[i]+s.introLen) || r.Intro.Confidence != 1 {
			t.Errorf("%s: intro %+v, want %v+%v with confidence 1", e.ID, r.Intro, s.introAt[i], s.introLen)
		}
		if !near(r.Outro, e.Duration-s.outroLen, e.Duration) {
			t.Errorf("%s: outro %+v", e.ID, r.Outro)
		}
	}
	// One episode alone has nothing to compare with.
	if res := Detect(eps[:1], DefaultOptions()); len(res) != 0 {
		t.Errorf("single episode: %+v", res)
	}
}

func TestDetectBounds(t *testing.T) {
	cases := []struct {
		name string
		s    season
	}{
		{"intro too short", season{duration: 5 * time.Minute, introLen: 10 * time.Second,
			introAt: []time.Duration{5 * time.Second, 20 * time.Second, 12 * time.Second}}},
		{"intro too long", season{duration: 20 * time.Minute, introLen: 4 * time.Minute,
			introAt: []time.Duration{5 * time.Second, 20 * time.Second, 12 * time.Second}}},
		{"outro far from the end", season{duration: 20 * time.Minute, introLen: 10 * time.Second, outroLen: 40 * time.Second,
			outroAhead: 30 * time.Second, introAt: []time.Duration{-1, -1, -1}}},
		{"outro too short", season{duration: 20 * time.Minute, introLen: 10 * time.Second, outroLen: 15 * time.Second,
			introAt: []time.Duration{-1, -1, -1}}},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if res := Detect(c.s.build(t, uint64(10+i)), DefaultOptions()); len(res) != 0 {
				t.Errorf("Detect = %+v, want nothing", res)
			}
		})
	}
}

func TestDetectMajority(t *testing.T) {
	// The first episode (a pilot) has no intro: the others agree with each other on theirs.
	s := season{duration: 20 * time.Minute, introLen: 30 * time.Second,
		introAt: []time.Duration{-1, 50 * time.Second, 20 * time.Second, 75 * time.Second, 33 * time.Second}}
	eps := s.build(t, 4)
	res := Detect(eps, DefaultOptions())
	if r, ok := res["e1"]; ok {
		t.Errorf("pilot: %+v, want nothing", r)
	}
	for i, e := range eps[1:] {
		r := res[e.ID]
		at := s.introAt[i+1]
		if !near(r.Intro, at, at+s.introLen) || r.Intro.Confidence != 0.75 {
			t.Errorf("%s: intro %+v, want %v with confidence 0.75", e.ID, r.Intro, at)
		}
	}
}

func TestDetectSilenceIsNoIntro(t *testing.T) {
	// Long identical stretches of one value (silence, a held tone) match but carry no position.
	o := DefaultOptions()
	f := newFingerprints(5)
	var eps []Episode
	for i := range 4 {
		d := 10 * time.Minute
		e := Episode{ID: fmt.Sprint(i), Duration: d, Head: f.noise(o.HeadLength(d))}
		for k := 0; k < int(40*time.Second/Step); k++ {
			e.Head[int(time.Duration(i+1)*10*time.Second/Step)+k] = 0x5555aaaa
		}
		eps = append(eps, e)
	}
	if res := Detect(eps, o); len(res) != 0 {
		t.Errorf("Detect = %+v, want nothing", res)
	}
}

func TestDetectChapters(t *testing.T) {
	s := season{duration: 20 * time.Minute, introLen: 30 * time.Second, outroLen: 45 * time.Second,
		introAt: []time.Duration{40 * time.Second, 50 * time.Second, 20 * time.Second}}
	eps := s.build(t, 6)
	eps[1].Chapters = []Chapter{
		{Start: 0, End: 50 * time.Second, Title: "Prolog"},
		{Start: 51 * time.Second, End: 80 * time.Second, Title: "Opening Credits"},
		{Start: 80 * time.Second, End: 19 * time.Minute, Title: "Kapitel 2"},
		{Start: 19 * time.Minute, End: eps[1].Duration, Title: "Abspann"},
	}
	// Chapters without such a title leave the fingerprints in charge.
	eps[2].Chapters = []Chapter{{Start: 0, End: 10 * time.Minute, Title: "Teil 1"}, {Start: 10 * time.Minute, End: eps[2].Duration, Title: "Teil 2"}}
	res := Detect(eps, DefaultOptions())
	r := res["e2"]
	want := Span{Start: 51 * time.Second, End: 80 * time.Second, Source: SourceChapter, Confidence: 1}
	if r.Intro == nil || *r.Intro != want {
		t.Errorf("chapter intro = %+v, want %+v", r.Intro, want)
	}
	if r.Outro == nil || r.Outro.Source != SourceChapter || r.Outro.Start != 19*time.Minute {
		t.Errorf("chapter outro = %+v", r.Outro)
	}
	if r := res["e3"]; r.Intro == nil || r.Intro.Source != SourceFingerprint || r.Outro == nil || r.Outro.Source != SourceFingerprint {
		t.Errorf("e3 = %+v, %+v; want fingerprint segments", r.Intro, r.Outro)
	}
	// Chapters alone, without any fingerprint, still count.
	only := []Episode{{ID: "x", Duration: time.Hour, Chapters: []Chapter{{Start: time.Minute, End: 2 * time.Minute, Title: "INTRO"}}}}
	if r := Detect(only, DefaultOptions())["x"]; r.Intro == nil || r.Intro.Start != time.Minute || r.Outro != nil {
		t.Errorf("chapters only = %+v", r)
	}
}

func TestChapterKind(t *testing.T) {
	for title, want := range map[string]string{
		"Intro": "intro", "Opening": "intro", "Opening Credits": "intro", "Vorspann": "intro", "Main Titles": "intro",
		"Credits": "outro", "End Credits": "outro", "Abspann": "outro", "Ending": "outro", "Outro": "outro", "End Titles": "outro",
		"Introduction": "", "Kapitel 1": "", "": "", "Chapter 3": "", "Recap": "",
	} {
		if got := ChapterKind(title); got != want {
			t.Errorf("ChapterKind(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestWindows(t *testing.T) {
	o := DefaultOptions()
	if got := o.HeadLength(22 * time.Minute); got != 528*time.Second {
		t.Errorf("head of 22 min = %v, want 40 %%", got)
	}
	if got := o.HeadLength(60 * time.Minute); got != 10*time.Minute {
		t.Errorf("head of 60 min = %v, want 10 min", got)
	}
	if from, length := o.TailRange(45 * time.Minute); from != 39*time.Minute || length != 6*time.Minute {
		t.Errorf("tail of 45 min = %v+%v", from, length)
	}
	if from, length := o.TailRange(100 * time.Second); from != 0 || length != 100*time.Second {
		t.Errorf("tail of 100 s = %v+%v", from, length)
	}
}
