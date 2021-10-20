package storage

import (
	"bytes"
	"github.com/mixanemca/pdns-api/internal/domain/forwardzone"
	"github.com/mixanemca/pdns-api/internal/infrastructure/errors"
	"os"
	"os/exec"
)

type FSStorage struct {
	path string
}

func NewFSStorage(path string) *FSStorage {
	return &FSStorage{path: path}
}

func (s *FSStorage) Save(fzs []forwardzone.ForwardZone) error {
	file, err := os.OpenFile(s.path, os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return errors.Wrap(err, "writing forward-zones-file")
	}
	defer file.Close()

	var buf bytes.Buffer
	for _, fz := range fzs {
		buf.WriteString("+" + fz.String())
	}
	_, err = buf.WriteTo(file)
	if err != nil {
		return errors.Wrap(err, "writing forward-zones-file")
	}
	cmd := exec.Command("systemctl", "restart", "pdns-recursor")
	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "writing forward-zones-file")
	}
	return nil
}
