package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func openTestCache(t *testing.T, server *miniredis.Miniredis) *Cache {
	t.Helper()
	t.Setenv("REDIS_ADDR", server.Addr())
	t.Setenv("REDIS_DB", "0")
	c, err := Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestSetGetDelete(t *testing.T) {
	server := miniredis.RunT(t)
	c := openTestCache(t, server)
	ctx := context.Background()

	if _, found, err := c.Get(ctx, "k"); err != nil || found {
		t.Fatalf("missing key: found=%v err=%v", found, err)
	}
	if err := c.Set(ctx, "k", []byte(`{"a":1}`), 30*time.Second); err != nil {
		t.Fatal(err)
	}
	if value, found, err := c.Get(ctx, "k"); err != nil || !found || string(value) != `{"a":1}` {
		t.Fatalf("got %q found=%v err=%v", value, found, err)
	}
	if ttl := server.TTL("k"); ttl != 30*time.Second {
		t.Fatalf("TTL = %v, want 30s", ttl)
	}
	server.FastForward(31 * time.Second)
	if _, found, _ := c.Get(ctx, "k"); found {
		t.Fatal("key must expire after its TTL")
	}

	if err := c.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := c.Delete(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := c.Get(ctx, "k"); found {
		t.Fatal("key was not deleted")
	}
}

func TestDeletePrefix(t *testing.T) {
	server := miniredis.RunT(t)
	c := openTestCache(t, server)
	ctx := context.Background()
	for _, key := range []string{"report:summary:a", "report:summary:b", "budgets:all"} {
		if err := c.Set(ctx, key, []byte("v"), time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.DeletePrefix(ctx, "report:summary:"); err != nil {
		t.Fatal(err)
	}
	if keys := server.Keys(); len(keys) != 1 || keys[0] != "budgets:all" {
		t.Fatalf("unexpected keys after DeletePrefix: %v", keys)
	}
}

func TestOpenErrors(t *testing.T) {
	t.Run("redis is down", func(t *testing.T) {
		server := miniredis.RunT(t)
		t.Setenv("REDIS_ADDR", server.Addr())
		server.Close()
		if _, err := Open(context.Background()); err == nil {
			t.Fatal("expected a connection error")
		}
	})
	t.Run("bad REDIS_DB", func(t *testing.T) {
		t.Setenv("REDIS_DB", "abc")
		if _, err := Open(context.Background()); err == nil {
			t.Fatal("expected an error for REDIS_DB=abc")
		}
	})
}

func TestPasswordAndDB(t *testing.T) {
	server := miniredis.RunT(t)
	server.RequireAuth("secret")
	t.Setenv("REDIS_ADDR", server.Addr())

	if _, err := Open(context.Background()); err == nil {
		t.Fatal("connecting without the password must fail")
	}
	t.Setenv("REDIS_PASSWORD", "secret")
	t.Setenv("REDIS_DB", "3")
	c, err := Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.Set(context.Background(), "k", []byte("v"), time.Minute); err != nil {
		t.Fatal(err)
	}
	server.Select(3)
	if !server.Exists("k") {
		t.Fatal("key must be stored in the database from REDIS_DB")
	}
}
