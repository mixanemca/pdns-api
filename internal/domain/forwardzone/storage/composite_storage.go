package storage

import "github.com/mixanemca/pdns-api/internal/domain/forwardzone"

type CompositeStorage struct {
	storages []Storage
}

func NewCompositeStorage(storages []Storage) *CompositeStorage {
	return &CompositeStorage{storages: storages}
}

func (s *CompositeStorage) Save(fzs []forwardzone.ForwardZone) error {
	for _, storage := range s.storages {
		err := storage.Save(fzs)
		if err != nil {
			return err
		}
	}

	return nil
}
