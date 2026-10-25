package app

import (
	"siagars/backend/internal/config"
	"siagars/backend/internal/repository"
)

// enableBPJSHistory turns on encrypted storage of BPJS check input when a server key is configured.
func enableBPJSHistory(repo *repository.Mongo, cfg config.Config) error {
	if cfg.BPJSHistoryKey == nil {
		return nil
	}
	c, e := repository.NewHistoryCipher(cfg.BPJSHistoryKey)
	if e != nil {
		return e
	}
	repo.SetHistoryCipher(c)
	return nil
}
