package persistence

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type idempotencyFile struct {
	SchemaVersion int                     `json:"schemaVersion"`
	Entries       map[string]CommitResult `json:"entries"`
}

func (r *FileRepository) loadIdempotency() error {
	data, err := os.ReadFile(r.idemPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var file idempotencyFile
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}
	if file.SchemaVersion != schemaVersion {
		return errors.New("幂等索引 schemaVersion 不兼容")
	}
	if file.Entries != nil {
		r.idempotency = file.Entries
	}
	return nil
}

func (r *FileRepository) writeIdempotency() error {
	temporary, err := os.CreateTemp(r.dir, ".idempotency-*.tmp")
	if err != nil {
		return err
	}
	path := temporary.Name()
	cleanup := func() { temporary.Close(); os.Remove(path) }
	encoder := json.NewEncoder(temporary)
	if err := encoder.Encode(idempotencyFile{SchemaVersion: schemaVersion, Entries: r.idempotency}); err != nil {
		cleanup()
		return err
	}
	if err := temporary.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := temporary.Close(); err != nil {
		os.Remove(path)
		return err
	}
	if err := os.Rename(path, r.idemPath); err != nil {
		os.Remove(path)
		return err
	}
	directory, err := os.Open(filepath.Dir(r.idemPath))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
