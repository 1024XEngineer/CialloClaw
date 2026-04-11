package plugin

import "testing"

func TestServiceSidecars(t *testing.T) {
	service := NewService()
	if len(service.Sidecars()) != 1 || service.Sidecars()[0] != "playwright_sidecar" {
		t.Fatalf("unexpected sidecars: %+v", service.Sidecars())
	}
	if !service.HasSidecar("playwright_sidecar") {
		t.Fatal("expected playwright_sidecar to be declared")
	}
	if service.PrimarySidecar() != "playwright_sidecar" {
		t.Fatalf("unexpected primary sidecar: %q", service.PrimarySidecar())
	}
}
