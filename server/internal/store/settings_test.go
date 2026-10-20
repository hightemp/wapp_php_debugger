package store

import (
	"context"
	"testing"
)

func TestSettingsAreKeptAsJSON(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if _, found, err := s.Setting(ctx, "desktop"); err != nil || found {
		t.Fatalf("empty: found=%v err=%v", found, err)
	}
	if err := s.SetSetting(ctx, "desktop", []byte(`{"autostart":true}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting(ctx, "desktop", []byte(`{"autostart":false}`)); err != nil {
		t.Fatal(err)
	}
	value, found, err := s.Setting(ctx, "desktop")
	if err != nil || !found || string(value) != `{"autostart":false}` {
		t.Fatalf("Setting = %s, %v, %v", value, found, err)
	}
	if err := s.SetSetting(ctx, "desktop", []byte(`not json`)); err == nil {
		t.Fatal("invalid JSON stored")
	}
}
