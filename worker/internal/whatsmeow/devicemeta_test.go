package whatsmeow

import (
	"path/filepath"
	"testing"
)

func newDeviceMeta(t *testing.T) *DeviceMetaStore {
	t.Helper()
	dir := t.TempDir()
	store, err := OpenDeviceMeta(filepath.Join(dir, "meta.db"))
	if err != nil {
		t.Fatalf("OpenDeviceMeta: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	return store
}

func TestDeviceMetaStoreSetGet(t *testing.T) {
	store := newDeviceMeta(t)

	if err := store.Set("5511999999999.0:1@s.whatsapp.net", 42); err != nil {
		t.Fatalf("Set: %v", err)
	}

	connID, ok, err := store.GetConnID("5511999999999.0:1@s.whatsapp.net")
	if err != nil {
		t.Fatalf("GetConnID: %v", err)
	}
	if !ok || connID != 42 {
		t.Fatalf("got connID=%d ok=%v, want 42 true", connID, ok)
	}

	jid, ok, err := store.GetJID(42)
	if err != nil {
		t.Fatalf("GetJID: %v", err)
	}
	if !ok || jid != "5511999999999.0:1@s.whatsapp.net" {
		t.Fatalf("got jid=%q ok=%v", jid, ok)
	}
}

func TestDeviceMetaStoreUpsertOnSet(t *testing.T) {
	store := newDeviceMeta(t)

	if err := store.Set("a@s.whatsapp.net", 1); err != nil {
		t.Fatalf("first Set: %v", err)
	}
	if err := store.Set("a@s.whatsapp.net", 2); err != nil {
		t.Fatalf("second Set: %v", err)
	}
	connID, ok, err := store.GetConnID("a@s.whatsapp.net")
	if err != nil || !ok {
		t.Fatalf("GetConnID err=%v ok=%v", err, ok)
	}
	if connID != 2 {
		t.Errorf("got connID=%d, want 2", connID)
	}
}

func TestDeviceMetaStoreMissing(t *testing.T) {
	store := newDeviceMeta(t)

	connID, ok, err := store.GetConnID("missing@s.whatsapp.net")
	if err != nil {
		t.Fatalf("GetConnID error: %v", err)
	}
	if ok || connID != 0 {
		t.Errorf("expected miss, got connID=%d ok=%v", connID, ok)
	}
}

func TestDeviceMetaStoreDelete(t *testing.T) {
	store := newDeviceMeta(t)
	if err := store.Set("a@s.whatsapp.net", 1); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := store.Delete("a@s.whatsapp.net"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, ok, _ := store.GetConnID("a@s.whatsapp.net")
	if ok {
		t.Errorf("expected gone after Delete")
	}
}

func TestDeviceMetaStoreDeleteByConnID(t *testing.T) {
	store := newDeviceMeta(t)
	if err := store.Set("b@s.whatsapp.net", 7); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := store.DeleteByConnID(7); err != nil {
		t.Fatalf("DeleteByConnID: %v", err)
	}
	_, ok, _ := store.GetConnID("b@s.whatsapp.net")
	if ok {
		t.Errorf("expected row gone after DeleteByConnID")
	}
}
