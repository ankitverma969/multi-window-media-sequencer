package models

import (
	"encoding/json"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMediaTypeValidation(t *testing.T) {
	validTypes := []MediaType{MediaTypeVideo, MediaTypeImage, MediaTypeBlank}
	for _, mt := range validTypes {
		if !mt.IsValid() {
			t.Errorf("expected MediaType %s to be valid", mt)
		}
	}

	invalidType := MediaType("audio")
	if invalidType.IsValid() {
		t.Errorf("expected MediaType audio to be invalid")
	}
}

func TestMediaValidation(t *testing.T) {
	valid := &Media{
		MediaKey:        "M1",
		Name:            "Nature Video",
		Type:            MediaTypeVideo,
		URL:             "https://example.com/video.mp4",
		DurationSeconds: 30,
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("expected valid media, got: %v", err)
	}

	blank := &Media{
		MediaKey:        "BLANK",
		Name:            "Configured Blank Screen",
		Type:            MediaTypeBlank,
		URL:             "", // Blank does not require URL
		DurationSeconds: 10,
	}
	if err := blank.Validate(); err != nil {
		t.Errorf("expected valid blank media, got: %v", err)
	}

	invalidDuration := &Media{
		MediaKey:        "M2",
		Name:            "Promo Image",
		Type:            MediaTypeImage,
		URL:             "https://example.com/img.jpg",
		DurationSeconds: 0,
	}
	if err := invalidDuration.Validate(); err == nil {
		t.Errorf("expected error for 0 duration")
	}
}

func TestWindowValidation(t *testing.T) {
	w := &Window{
		WindowNumber:         1,
		Name:                 "Window 1",
		CycleDurationSeconds: 18000,
		CycleStartTime:       time.Now().UTC(),
		IsActive:             true,
	}
	if err := w.Validate(); err != nil {
		t.Errorf("expected valid window, got: %v", err)
	}

	invalidW := &Window{
		WindowNumber: 0,
		Name:         "Invalid",
	}
	if err := invalidW.Validate(); err == nil {
		t.Errorf("expected error for window number 0")
	}
}

func TestPlaylistRecalculate(t *testing.T) {
	p := &Playlist{
		WindowNumber: 1,
		Items: []PlaylistItem{
			{ItemID: "1", MediaKey: "M1", Type: MediaTypeVideo, DurationSeconds: 30},
			{ItemID: "2", MediaKey: "M2", Type: MediaTypeImage, DurationSeconds: 15},
			{ItemID: "3", MediaKey: "M3", Type: MediaTypeVideo, DurationSeconds: 45},
		},
		Version: 1,
	}

	p.Recalculate()

	if p.TotalSequenceDurationSeconds != 90 {
		t.Errorf("expected total duration 90, got %d", p.TotalSequenceDurationSeconds)
	}
	if p.Version != 2 {
		t.Errorf("expected version 2, got %d", p.Version)
	}
	if p.Items[0].Order != 1 || p.Items[1].Order != 2 || p.Items[2].Order != 3 {
		t.Errorf("expected sequential orders 1, 2, 3")
	}
}

func TestSyncEventValidation(t *testing.T) {
	now := time.Now().UTC()
	validSync := &SyncEvent{
		EventID:         "sync_123",
		MediaKey:        "M2",
		DurationSeconds: 15,
		StartTime:       now,
		EndTime:         now.Add(15 * time.Second),
		Status:          SyncStatusScheduled,
	}
	if err := validSync.Validate(); err != nil {
		t.Errorf("expected valid sync event, got: %v", err)
	}

	invalidTimeSync := &SyncEvent{
		EventID:         "sync_123",
		MediaKey:        "M2",
		DurationSeconds: 15,
		StartTime:       now,
		EndTime:         now.Add(-5 * time.Second), // End before start
	}
	if err := invalidTimeSync.Validate(); err == nil {
		t.Errorf("expected error when end_time is before start_time")
	}
}

func TestModelJSONSerialization(t *testing.T) {
	oid := bson.NewObjectID()
	m := Media{
		ID:              oid,
		MediaKey:        "M1",
		Name:            "Video 1",
		Type:            MediaTypeVideo,
		URL:             "https://example.com/v.mp4",
		DurationSeconds: 20,
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded Media
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.MediaKey != m.MediaKey || decoded.DurationSeconds != m.DurationSeconds {
		t.Errorf("decoded media does not match original: %+v vs %+v", decoded, m)
	}
}
