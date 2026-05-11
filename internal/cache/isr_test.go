package cache

import (
	"testing"
	"time"
)

func TestISRSetGetFresh(t *testing.T) {
	c := NewISR("")
	defer c.Close()

	c.Set("/about", "<html>about</html>", 60)
	html, stale, err := c.Get("/about")
	if err != nil {
		t.Fatalf("expected cache hit, got error: %v", err)
	}
	if stale {
		t.Fatalf("expected fresh cache entry")
	}
	if html != "<html>about</html>" {
		t.Fatalf("unexpected html: %s", html)
	}
}

func TestISRSetGetStale(t *testing.T) {
	c := NewISR("")
	defer c.Close()

	c.Set("/blog/post", "<html>post</html>", 1)
	time.Sleep(1100 * time.Millisecond)

	html, stale, err := c.Get("/blog/post")
	if err != nil {
		t.Fatalf("expected stale hit, got error: %v", err)
	}
	if !stale {
		t.Fatalf("expected stale entry")
	}
	if html == "" {
		t.Fatalf("expected stale html to be served")
	}
}

func TestISRBackgroundRevalidate(t *testing.T) {
	c := NewISR("")
	defer c.Close()

	c.Set("/", "old", 1)
	time.Sleep(1100 * time.Millisecond)

	done := make(chan struct{})
	c.BackgroundRevalidate("/", func(path string) (string, int, []string) {
		close(done)
		return "new", 30, []string{"home"}
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("background revalidation timeout")
	}

	html, stale, err := c.Get("/")
	if err != nil {
		t.Fatalf("expected cache hit after revalidate: %v", err)
	}
	if stale {
		t.Fatalf("expected fresh entry after revalidate")
	}
	if html != "new" {
		t.Fatalf("expected updated html, got %q", html)
	}
}
