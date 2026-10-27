package app

import (
	"siagars/backend/internal/config"
	"siagars/backend/internal/repository"
)

// enableBPJSHistory turns on the keyed NIK fingerprint of BPJS history when a server key is configured.
func enableBPJSHistory(repo *repository.Mongo, cfg config.Config) error {
	if cfg.BPJSHistoryKey == nil {
		return nil
	}
	c, e := repository.NewHistoryHasher(cfg.BPJSHistoryKey)
	if e != nil {
		return e
	}
	repo.SetHistoryHasher(c)
	return nil
}
