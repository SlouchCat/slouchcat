package main

// The structures of this file mirror the schemas of api/openapi.yaml that the client reads. They
// are written by hand on purpose: a third-party client has the contract, not the generated code
// of this repository.

type sessionInfo struct {
	Account struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
		Role  string `json:"role"`
	} `json:"account"`
	Profile *struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		IsPrimary bool   `json:"isPrimary"`
		MaxRating int    `json:"maxRating"`
	} `json:"profile"`
}

type profileSummary struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	HasPin    bool   `json:"hasPin"`
	IsPrimary bool   `json:"isPrimary"`
}

type library struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	DefaultRating *int   `json:"defaultRating,omitempty"`
	Scan          struct {
		State     string `json:"state"`
		LastError string `json:"lastError"`
		ItemCount int    `json:"itemCount"`
	} `json:"scan"`
}

type libraryRequest struct {
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	Path          string `json:"path,omitempty"`
	DefaultRating *int   `json:"defaultRating,omitempty"`
}

type itemRef struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind"`
	Title         string    `json:"title"`
	Year          int       `json:"year"`
	DurationMs    int64     `json:"durationMs"`
	Explicit      bool      `json:"explicit"`
	SeriesTitle   string    `json:"seriesTitle"`
	EpisodeNumber int       `json:"episodeNumber"`
	Progress      *progress `json:"progress"`
}

type progress struct {
	PositionMs int64   `json:"positionMs"`
	DurationMs int64   `json:"durationMs"`
	Percent    float64 `json:"percent"`
	Completed  bool    `json:"completed"`
}

type mediaFileInfo struct {
	ID         string `json:"id"`
	Container  string `json:"container"`
	VideoCodec string `json:"videoCodec"`
	AudioCodec string `json:"audioCodec"`
	DurationMs int64  `json:"durationMs"`
	SizeBytes  int64  `json:"sizeBytes"`
	Available  bool   `json:"available"`
}

type itemDetail struct {
	itemRef
	Overview     string          `json:"overview"`
	Genres       []string        `json:"genres"`
	InWatchlist  bool            `json:"inWatchlist"`
	Favorite     bool            `json:"favorite"`
	MediaFiles   []mediaFileInfo `json:"mediaFiles"`
	Chapters     []chapter       `json:"chapters"`
	Episodes     []itemRef       `json:"episodes"`
	EpisodeCount int             `json:"episodeCount"`
	Seasons      []itemRef       `json:"seasons"`
	TrailerCount int             `json:"trailerCount"`
	Narrator     string          `json:"narrator"`
}

type chapter struct {
	StartMs int64  `json:"startMs"`
	EndMs   int64  `json:"endMs"`
	Title   string `json:"title"`
}

type homePage struct {
	Rows []struct {
		Key    string    `json:"key"`
		Items  []itemRef `json:"items"`
		Genres []struct {
			Name string `json:"name"`
		} `json:"genres"`
	} `json:"rows"`
}

type searchRequest struct {
	Query        string   `json:"query,omitempty"`
	Genre        string   `json:"genre,omitempty"`
	YearFrom     int      `json:"yearFrom,omitempty"`
	YearTo       int      `json:"yearTo,omitempty"`
	MaxRating    *int     `json:"maxRating,omitempty"`
	MediaTypes   []string `json:"mediaTypes,omitempty"`
	Availability []string `json:"availability,omitempty"`
	Limit        int      `json:"limit,omitempty"`
}

type searchResult struct {
	Items  []itemRef `json:"items"`
	Total  int       `json:"total"`
	People []struct {
		Name string `json:"name"`
	} `json:"people"`
}

type track struct {
	ID       int64  `json:"id"`
	Index    int    `json:"index"`
	Kind     string `json:"kind"`
	Codec    string `json:"codec"`
	Language string `json:"language"`
	Title    string `json:"title"`
	Default  bool   `json:"default"`
	Text     bool   `json:"text"`
	Channels int    `json:"channels"`
}

type trackList struct {
	MediaFileID string  `json:"mediaFileId"`
	Audio       []track `json:"audio"`
	Subtitles   []track `json:"subtitles"`
}

type playbackCapabilities struct {
	Containers []string `json:"containers,omitempty"`
	Video      []string `json:"video"`
	Audio      []string `json:"audio"`
	HLS        string   `json:"hls"`
	MaxWidth   int      `json:"maxWidth,omitempty"`
	MaxHeight  int      `json:"maxHeight,omitempty"`
	Name       string   `json:"name,omitempty"`
	Type       string   `json:"type,omitempty"`
}

type playbackRequest struct {
	MediaFileID      string                `json:"mediaFileId,omitempty"`
	DeviceID         string                `json:"deviceId,omitempty"`
	Capabilities     *playbackCapabilities `json:"capabilities,omitempty"`
	AudioTrack       *int                  `json:"audioTrack,omitempty"`
	SubtitleTrack    *int64                `json:"subtitleTrack,omitempty"`
	SubtitleDelivery string                `json:"subtitleDelivery,omitempty"`
	Quality          string                `json:"quality,omitempty"`
	PositionMs       *int64                `json:"positionMs,omitempty"`
}

type playbackInfo struct {
	Mode              string        `json:"mode"`
	MediaKind         string        `json:"mediaKind"`
	Item              itemRef       `json:"item"`
	MediaFile         mediaFileInfo `json:"mediaFile"`
	StreamURL         string        `json:"streamUrl"`
	SubtitleURL       string        `json:"subtitleUrl"`
	PlaybackSessionID string        `json:"playbackSessionId"`
	ResumePositionMs  int64         `json:"resumePositionMs"`
	Tracks            trackList     `json:"tracks"`
	Selection         struct {
		AudioTrack       *int   `json:"audioTrack"`
		SubtitleTrack    *int64 `json:"subtitleTrack"`
		SubtitleDelivery string `json:"subtitleDelivery"`
	} `json:"selection"`
	Qualities []struct {
		Quality   string `json:"quality"`
		Available bool   `json:"available"`
		Current   bool   `json:"current"`
	} `json:"qualities"`
	Chapters    []chapter `json:"chapters"`
	Transcoding *struct {
		Mode     string `json:"mode"`
		Reason   string `json:"reason"`
		Backend  string `json:"backend"`
		Hardware bool   `json:"hardware"`
	} `json:"transcoding"`
}

type progressEvent struct {
	MediaFileID       string `json:"mediaFileId,omitempty"`
	PlaybackSessionID string `json:"playbackSessionId,omitempty"`
	PositionMs        int64  `json:"positionMs"`
	DurationMs        int64  `json:"durationMs"`
	PlayedMs          int64  `json:"playedMs"`
	Seq               int64  `json:"seq"`
	State             string `json:"state"`
	ClientTime        string `json:"clientTime"`
}

type downloadOptions struct {
	MediaFileID string `json:"mediaFileId"`
	DurationMs  int64  `json:"durationMs"`
	Qualities   []struct {
		Quality        string `json:"quality"`
		EstimatedBytes int64  `json:"estimatedBytes"`
		Available      bool   `json:"available"`
	} `json:"qualities"`
	AudioTracks []track `json:"audioTracks"`
	Subtitles   []track `json:"subtitles"`
}

type downloadRequest struct {
	Quality     string `json:"quality"`
	AudioTrack  *int   `json:"audioTrack,omitempty"`
	MediaFileID string `json:"mediaFileId,omitempty"`
}

type downloadState struct {
	ID                string   `json:"id"`
	State             string   `json:"state"`
	Quality           string   `json:"quality"`
	AllowedProfileIDs []int64  `json:"allowedProfileIds"`
	EstimatedBytes    int64    `json:"estimatedBytes"`
	SizeBytes         int64    `json:"sizeBytes"`
	SegmentsDone      int      `json:"segmentsDone"`
	SegmentsTotal     int      `json:"segmentsTotal"`
	Error             string   `json:"error"`
	Item              *itemRef `json:"item"`
}

type downloadList struct {
	Downloads []downloadState `json:"downloads"`
}

type downloadManifest struct {
	ID         string `json:"id"`
	ItemID     string `json:"itemId"`
	SizeBytes  int64  `json:"sizeBytes"`
	DurationMs int64  `json:"durationMs"`
	MimeType   string `json:"mimeType"`
	Codecs     string `json:"codecs"`
	InitLength int64  `json:"initLength"`
	Segments   []struct {
		StartMs    int64 `json:"startMs"`
		DurationMs int64 `json:"durationMs"`
		Offset     int64 `json:"offset"`
		Length     int64 `json:"length"`
	} `json:"segments"`
	FileURL   string    `json:"fileUrl"`
	Chapters  []chapter `json:"chapters"`
	Subtitles []struct {
		Track track  `json:"track"`
		URL   string `json:"url"`
	} `json:"subtitles"`
}

type offlineEvent struct {
	ProfileID  int64  `json:"profileId"`
	ItemID     string `json:"itemId"`
	PlaybackID string `json:"playbackId"`
	DownloadID string `json:"downloadId,omitempty"`
	PositionMs int64  `json:"positionMs"`
	DurationMs int64  `json:"durationMs"`
	PlayedMs   int64  `json:"playedMs"`
	Seq        int64  `json:"seq"`
	State      string `json:"state"`
	ClientTime string `json:"clientTime"`
}

type offlineEventBatch struct {
	SentAt string         `json:"sentAt"`
	Events []offlineEvent `json:"events"`
}

type offlineEventResult struct {
	Accepted   int      `json:"accepted"`
	Duplicates int      `json:"duplicates"`
	Rejected   int      `json:"rejected"`
	Results    []string `json:"results"`
}

type notificationPage struct {
	Notifications []struct {
		ID     int64    `json:"id"`
		Kind   string   `json:"kind"`
		ReadAt string   `json:"readAt"`
		Item   itemRef  `json:"item"`
		Parent *itemRef `json:"parent"`
	} `json:"notifications"`
	Total  int `json:"total"`
	Unread int `json:"unread"`
}

type announcement struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	PublishedAt string `json:"publishedAt"`
}

type announcementRequest struct {
	Title   string `json:"title"`
	Body    string `json:"body"`
	Publish bool   `json:"publish"`
}

type trailer struct {
	ID        int64  `json:"id"`
	Lang      string `json:"lang"`
	YoutubeID string `json:"youtubeId"`
	EmbedURL  string `json:"embedUrl"`
	Source    string `json:"source"`
	Official  bool   `json:"official"`
}

type trailerRequest struct {
	Lang      string `json:"lang"`
	YoutubeID string `json:"youtubeId"`
}

type podcastCreate struct {
	LibraryID int64  `json:"libraryId"`
	FeedURL   string `json:"feedUrl"`
}

type adminPodcast struct {
	ID           string `json:"id"`
	LibraryID    int64  `json:"libraryId"`
	Title        string `json:"title"`
	FeedURL      string `json:"feedUrl"`
	EpisodeCount int    `json:"episodeCount"`
	LastError    string `json:"lastError"`
}

type setupStatus struct {
	SetupRequired bool `json:"setupRequired"`
}

type setupRequest struct {
	Email    string         `json:"email"`
	Password string         `json:"password"`
	Language string         `json:"language"`
	Library  libraryRequest `json:"library"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type pinRequest struct {
	Pin string `json:"pin"`
}

type health struct {
	Status string `json:"status"`
}
