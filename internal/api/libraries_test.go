package api

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestCollectPagesWalksEveryPage(t *testing.T) {
	t.Parallel()
	pages := map[string]Page[string]{
		"":   {Items: []string{"a", "b"}, NextPageToken: "t1"},
		"t1": {Items: []string{"c"}, NextPageToken: "t2"},
		"t2": {Items: []string{"d"}},
	}
	var asked []string
	var counts []int
	got, err := collectPages(context.Background(), func(token string) (Page[string], error) {
		asked = append(asked, token)
		return pages[token], nil
	}, func(count int) { counts = append(counts, count) })
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if fmt.Sprint(got) != "[a b c d]" {
		t.Fatalf("items=%v", got)
	}
	if fmt.Sprint(asked) != "[ t1 t2]" {
		t.Fatalf("tokens=%q", asked)
	}
	if fmt.Sprint(counts) != "[2 3 4]" {
		t.Fatalf("progress counts=%v", counts)
	}
}

func TestCollectPagesStopsOnRepeatedToken(t *testing.T) {
	t.Parallel()
	calls := 0
	got, err := collectPages(context.Background(), func(string) (Page[string], error) {
		calls++
		return Page[string]{Items: []string{"x"}, NextPageToken: "same"}, nil
	}, nil)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	// First page returns "same"; the second returns it again, ending the walk.
	if calls != 2 || len(got) != 2 {
		t.Fatalf("calls=%d items=%v", calls, got)
	}
}

func TestCollectPagesPropagatesError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("boom")
	if _, err := collectPages(context.Background(), func(string) (Page[string], error) {
		return Page[string]{}, sentinel
	}, nil); !errors.Is(err, sentinel) {
		t.Fatalf("err=%v", err)
	}
}

func TestCollectPagesHonoursContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	_, err := collectPages(ctx, func(string) (Page[string], error) {
		calls++
		cancel()
		return Page[string]{Items: []string{"x"}, NextPageToken: "next"}, nil
	}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d, should stop after cancellation", calls)
	}
}

func TestVersionStringsDedupesAndSkipsEmpty(t *testing.T) {
	t.Parallel()
	in := []LibraryArtifactVersion{
		{Version: "1.0.0", SourceType: "internal"},
		{Version: "1.0.0", SourceType: "remediated"}, // same version, other source
		{Version: ""},
		{Version: "2.0.0"},
	}
	got := versionStrings(in)
	if fmt.Sprint(got) != "[1.0.0 2.0.0]" {
		t.Fatalf("got %v", got)
	}
}
