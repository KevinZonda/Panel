package web

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestActionResultStoreRoundTrip(t *testing.T) {
	store := newActionResultStore()
	data := gin.H{"Title": "Nginx Action", "Result": "ok"}
	token, err := store.put("nginx", data)
	if err != nil {
		t.Fatalf("put action result: %v", err)
	}
	if token == "" {
		t.Fatal("empty action result token")
	}

	got, ok := store.get("nginx", token)
	if !ok {
		t.Fatal("stored action result not found")
	}
	if got["Title"] != "Nginx Action" || got["Result"] != "ok" {
		t.Fatalf("unexpected stored data: %#v", got)
	}
}

func TestActionResultStoreKeepsRequestDataSeparate(t *testing.T) {
	store := newActionResultStore()
	token, err := store.put("nginx", gin.H{"Result": "ok"})
	if err != nil {
		t.Fatalf("put action result: %v", err)
	}

	first, ok := store.get("nginx", token)
	if !ok {
		t.Fatal("stored action result not found")
	}
	first["CSRFToken"] = "first request"

	second, ok := store.get("nginx", token)
	if !ok || second["Result"] != "ok" {
		t.Fatalf("missing stored result: %#v", second)
	}
	if _, ok := second["CSRFToken"]; ok {
		t.Fatal("render data from the first request leaked into the second request")
	}
}

func TestActionResultStoreSeparatesKinds(t *testing.T) {
	store := newActionResultStore()
	token, err := store.put("nginx", gin.H{"Result": "ok"})
	if err != nil {
		t.Fatalf("put action result: %v", err)
	}
	if _, ok := store.get("frp", token); ok {
		t.Fatal("nginx result was visible through frp result endpoint")
	}
	if _, ok := store.get("nginx", token); !ok {
		t.Fatal("nginx result missing from nginx result endpoint")
	}
}

func TestActionResultStoreExpiresEntries(t *testing.T) {
	store := newActionResultStore()
	store.entries["expired"] = storedActionResult{
		kind:      "nginx",
		data:      gin.H{"Result": "old"},
		expiresAt: time.Now().Add(-time.Minute),
	}
	if _, ok := store.get("nginx", "expired"); ok {
		t.Fatal("expired action result is still visible")
	}
	if _, ok := store.entries["expired"]; ok {
		t.Fatal("expired action result was not removed")
	}
}
