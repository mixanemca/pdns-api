package storage

import "github.com/mixanemca/pdns-api/internal/domain/forwardzone"

type Storage interface {
	Save(fzs []forwardzone.ForwardZone) error
}
