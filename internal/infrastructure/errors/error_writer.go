package errors

import (
	"fmt"
	"net/http"

	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/mixanemca/pdns-api/internal/infrastructure"
	statistic "github.com/mixanemca/pdns-api/internal/infrastructure/stats"
	"github.com/sirupsen/logrus"
)

type errorWriter struct {
	config config.Config
	logger *logrus.Logger
	stats  statistic.PrometheusStatsCollector
}

func NewErrorWriter(config config.Config, logger *logrus.Logger, stats statistic.PrometheusStatsCollector) *errorWriter {
	return &errorWriter{config: config, logger: logger, stats: stats}
}

func (s *errorWriter) WriteError(w http.ResponseWriter, urlPath string, action string, err error) {
	var status int

	errorType := GetType(err)
	switch errorType {
	case BadRequest:
		status = http.StatusBadRequest
	case NotFound:
		status = http.StatusNotFound
	case Conflict:
		status = http.StatusConflict
	default:
		status = http.StatusInternalServerError
	}

	// Set response status
	w.WriteHeader(status)
	// Write error to response
	fmt.Fprintf(w, "Error: %s\n", err.Error())

	s.logger.WithFields(logrus.Fields{
		"action": action,
	}).Error(err.Error())

	s.stats.CountError(s.config.Environment, infrastructure.GetHostname(), urlPath, status)
}
