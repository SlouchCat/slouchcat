package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// lateLimit is the calibration criterion of plan 3.5: a segment that is ready more than this
// after its playback time means the server is over budget.
const lateLimit = time.Second

// hlsbench measures concurrent HLS sessions on one file for the calibration of costs and budgets
// (plan 3.5, docs/perf/M1b.md). Every stream reads its segments in order as fast as they come, like
// a player filling its buffer; playback is assumed to start when segment 0 is ready, so segment i
// is due at that time plus its start in the film.
//
//	slouchcat hlsbench --file F [--streams N] [--mode transcode|remux] [--hw auto] [--max-height 1080]
//	                   [--seconds 60] [--budget 1000] [--json]
func hlsbench(args []string) int {
	fs := flag.NewFlagSet("hlsbench", flag.ContinueOnError)
	file := fs.String("file", "", "media file to stream")
	streams := fs.Int("streams", 1, "concurrent sessions on the file")
	mode := fs.String("mode", "transcode", "transcode or remux")
	hw := fs.String("hw", "auto", "backend: auto, nvenc, qsv, vaapi or software")
	maxHeight := fs.Int("max-height", 1080, "output height limit of a transcode")
	seconds := fs.Int("seconds", 60, "film seconds each stream reads")
	budget := fs.Float64("budget", 1000, "admission budget (default: high enough to admit every stream)")
	asJSON := fs.Bool("json", false, "print JSON instead of a table")
	ffmpeg := fs.String("ffmpeg", "ffmpeg", "ffmpeg binary")
	ffprobe := fs.String("ffprobe", "ffprobe", "ffprobe binary")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *file == "" || *streams < 1 || *seconds < 1 || (*mode != "transcode" && *mode != "remux") {
		fmt.Fprintln(os.Stderr, "usage: slouchcat hlsbench --file F [--streams N] [--mode transcode|remux] [--hw auto] [--max-height 1080] [--seconds 60] [--budget 1000] [--json]")
		return 2
	}
	o := benchOptions{file: *file, streams: *streams, mode: transcode.Mode(*mode), hw: *hw, maxHeight: *maxHeight,
		seconds: time.Duration(*seconds) * time.Second, budget: *budget,
		runner: transcode.Runner{FFmpegPath: *ffmpeg, FFprobePath: *ffprobe}}
	rep, err := runHLSBench(context.Background(), o)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hlsbench:", err)
		return 1
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return 1
		}
	} else {
		printBenchTable(os.Stdout, rep)
	}
	return 0
}

type benchOptions struct {
	file      string
	streams   int
	mode      transcode.Mode
	hw        string
	maxHeight int
	seconds   time.Duration
	budget    float64
	runner    transcode.Runner
}

type benchReport struct {
	File     string        `json:"file"`
	Mode     string        `json:"mode"`
	Backend  string        `json:"backend"`
	FFmpeg   string        `json:"ffmpeg"`
	Streams  int           `json:"streams"`
	Seconds  float64       `json:"seconds"`
	Cost     float64       `json:"costPerStream"`
	Budget   float64       `json:"budget"`
	Size     string        `json:"output"`
	Results  []benchStream `json:"results"`
	AnyLate  bool          `json:"anyLate"`
	WallSecs float64       `json:"wallSeconds"`
}

type benchStream struct {
	Stream     int     `json:"stream"`
	Error      string  `json:"error,omitempty"`
	InitMS     int64   `json:"initMs"`
	FirstSegMS int64   `json:"firstSegmentMs"`
	Segments   int     `json:"segments"`
	MaxLagMS   int64   `json:"maxLagMs"`
	Late       bool    `json:"late"`
	CPUSeconds float64 `json:"cpuSeconds"`
	ReadMBytes float64 `json:"readMB"`
}

func runHLSBench(ctx context.Context, o benchOptions) (benchReport, error) {
	path, err := filepath.Abs(o.file)
	if err != nil {
		return benchReport{}, err
	}
	probe, err := o.runner.Probe(ctx, path)
	if err != nil {
		return benchReport{}, err
	}
	kf, err := o.runner.Keyframes(ctx, path)
	if err != nil {
		return benchReport{}, err
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	backend, caps, err := transcode.Detect(ctx, o.runner, o.hw, log)
	if err != nil {
		return benchReport{}, err
	}
	src := transcode.Source{Path: path, Probe: probe, Keyframes: kf}
	target := transcode.Target{Mode: o.mode, MaxHeight: o.maxHeight, Tonemap: true, SubtitleBurnIn: -1}
	if o.mode == transcode.ModeRemux {
		target = transcode.Target{Mode: o.mode, VideoCopy: true, SubtitleBurnIn: -1}
		for _, s := range probe.Streams {
			switch {
			case s.Type == "video" && s.Codec == "hevc" && !s.AttachedPic:
				target.VideoTag = "hvc1"
			case s.Type == "audio" && s.Codec == "aac":
				target.AudioCopy = true
			}
		}
	}
	plan, err := transcode.NewPlan(src, target)
	if err != nil {
		return benchReport{}, err
	}
	workDir, err := os.MkdirTemp("", "slouchcat-hlsbench-")
	if err != nil {
		return benchReport{}, err
	}
	defer os.RemoveAll(workDir)
	m, err := transcode.NewManager(transcode.ManagerOptions{
		Runner: o.runner, WorkDir: filepath.Join(workDir, "transcode"), Backend: backend, Caps: caps, Budget: o.budget, Log: log,
	})
	if err != nil {
		return benchReport{}, err
	}
	defer m.Close()

	rep := benchReport{
		File: path, Mode: string(o.mode), Backend: string(backend), FFmpeg: caps.Version, Streams: o.streams,
		Seconds: o.seconds.Seconds(), Cost: plan.Cost, Budget: m.Stats().Budget,
		Size: fmt.Sprintf("%dx%d", plan.Width, plan.Height), Results: make([]benchStream, o.streams),
	}
	if o.mode == transcode.ModeRemux {
		rep.Backend = "copy"
	}
	ctx, cancel := context.WithTimeout(ctx, 3*o.seconds+time.Minute)
	defer cancel()
	start := time.Now()
	var wg sync.WaitGroup
	for i := range o.streams {
		wg.Go(func() {
			rep.Results[i] = benchOne(ctx, m, i, src, plan, o.seconds, start)
		})
	}
	wg.Wait()
	rep.WallSecs = time.Since(start).Seconds()
	for _, r := range rep.Results {
		rep.AnyLate = rep.AnyLate || r.Late || r.Error != ""
	}
	return rep, nil
}

// benchOne plays one stream: init, then the segments up to the film time limit, in order.
func benchOne(ctx context.Context, m *transcode.Manager, n int, src transcode.Source, plan transcode.Plan,
	limit time.Duration, start time.Time) (res benchStream) {
	res.Stream = n + 1
	id := fmt.Sprintf("bench%03d", n+1)
	s, err := m.Start(ctx, id, src, plan)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer func() {
		m.Stop(id)
		res.CPUSeconds = s.CPUTime().Seconds()
	}()
	var read int64
	consume := func(rc io.ReadSeekCloser, err error) error {
		if err != nil {
			return err
		}
		defer rc.Close()
		nb, err := io.Copy(io.Discard, rc)
		read += nb
		return err
	}
	if err := consume(s.OpenInit(ctx)); err != nil {
		res.Error = err.Error()
		return res
	}
	res.InitMS = time.Since(start).Milliseconds()
	var first time.Duration
	for i, seg := range plan.Segments {
		if seg.Start >= limit {
			break
		}
		if err := consume(s.OpenSegment(ctx, i)); err != nil {
			res.Error = fmt.Sprintf("segment %d: %v", i, err)
			break
		}
		ready := time.Since(start)
		if i == 0 {
			first = ready
			res.FirstSegMS = ready.Milliseconds()
		}
		lag := ready - (first + seg.Start)
		res.MaxLagMS = max(res.MaxLagMS, lag.Milliseconds())
		res.Late = res.Late || lag > lateLimit
		res.Segments++
	}
	res.ReadMBytes = float64(read) / 1e6
	if res.Error != "" && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		res.Late = true
	}
	return res
}

func printBenchTable(w io.Writer, rep benchReport) {
	fmt.Fprintf(w, "file: %s\nmode: %s, backend: %s, output %s, %d stream(s), %.0f s each, cost %.2f per stream, budget %.2f\n%s\n\n",
		rep.File, rep.Mode, rep.Backend, rep.Size, rep.Streams, rep.Seconds, rep.Cost, rep.Budget, rep.FFmpeg)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "stream\tinit ms\tfirst seg ms\tsegments\tmax lag ms\tlate >1s\tffmpeg cpu s\tMB\terror\t")
	for _, r := range rep.Results {
		fmt.Fprintf(tw, "%d\t%d\t%d\t%d\t%d\t%v\t%.1f\t%.1f\t%s\t\n",
			r.Stream, r.InitMS, r.FirstSegMS, r.Segments, r.MaxLagMS, r.Late, r.CPUSeconds, r.ReadMBytes, r.Error)
	}
	_ = tw.Flush()
	fmt.Fprintf(w, "\nwall %.1f s, any segment late or failed: %v\n", rep.WallSecs, rep.AnyLate)
}
