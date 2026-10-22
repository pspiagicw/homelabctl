package catalog

import (
	"encoding/json"
	"testing"

	"github.com/pspiagicw/homelabctl/config"
)

// setupCatalog creates a catalog with one service whose check returns up.
func setupCatalog(up bool) (*Catalog, *[]string) {
	checked := &[]string{}
	c := NewCatalog(&config.Config{Services: config.ServicesConfig{
		{Name: "Jellyfin", URL: "http://192.168.10.91:8096", Group: "Media"},
	}})
	c.services[0].CheckFunc = func(url string) bool {
		*checked = append(*checked, url)
		return up
	}
	return c, checked
}

func TestRefreshMarksUp(t *testing.T) {
	c, checked := setupCatalog(true)

	c.Refresh()

	s := c.List().Services[0]
	if !s.LastPing {
		t.Errorf("LastPing = false, want true")
	}
	if s.LastSeen.IsZero() {
		t.Errorf("LastSeen not set for an up service")
	}
	// The check gets the full URL, not just the host.
	if len(*checked) != 1 || (*checked)[0] != "http://192.168.10.91:8096" {
		t.Errorf("checked %v, want the service URL once", *checked)
	}
}

func TestRefreshMarksDown(t *testing.T) {
	c, _ := setupCatalog(false)

	c.Refresh()

	s := c.List().Services[0]
	if s.LastPing {
		t.Errorf("LastPing = true, want false")
	}
	if !s.LastSeen.IsZero() {
		t.Errorf("LastSeen = %v, want zero for a service never seen up", s.LastSeen)
	}
}

// TestListIsSafeDuringRefresh reads (and JSON-encodes) the list while a
// refresh writes it; run with -race to catch unguarded access.
func TestListIsSafeDuringRefresh(t *testing.T) {
	c, _ := setupCatalog(true)

	done := make(chan struct{})
	go func() {
		c.Refresh()
		close(done)
	}()
	for range 50 {
		if _, err := json.Marshal(c.List()); err != nil {
			t.Fatalf("json.Marshal(List()) error = %v", err)
		}
	}
	<-done
}
