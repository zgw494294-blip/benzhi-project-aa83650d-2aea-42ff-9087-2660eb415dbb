package persistence

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"phonemereleasedesk/internal/domain"
)

func TestCommitSyncAndRecover(t *testing.T) {
	dir := t.TempDir()
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := domain.NewReleaseBatch("b", "点", "IPA", []string{"a"}, 100, true, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Commit(context.Background(), batch, 0, "create", "key", time.Unix(1, 0)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Close(); err != nil {
		t.Fatal(err)
	}
	recovered, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	loaded, err := recovered.Get(context.Background(), "b")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != 1 || recovered.sequence != 1 || recovered.lastHash == "" {
		t.Fatalf("恢复投影异常：%+v", loaded)
	}
	if _, err := os.Stat(filepath.Join(dir, "snapshots", "b.json")); err != nil {
		t.Fatal(err)
	}
}

func TestDetectsTamperedLedger(t *testing.T) {
	dir := t.TempDir()
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	batch, _ := domain.NewReleaseBatch("b", "点", "IPA", []string{"a"}, 100, true, time.Unix(1, 0))
	if _, err := repo.Commit(context.Background(), batch, 0, "create", "", time.Unix(1, 0)); err != nil {
		t.Fatal(err)
	}
	repo.Close()
	path := filepath.Join(dir, "events.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)/2] ^= 1
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("篡改后的账本通过了校验")
	}
}
