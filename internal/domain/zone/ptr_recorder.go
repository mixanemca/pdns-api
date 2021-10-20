package zone

import (
	"strings"

	"github.com/miekg/dns"
	pdnsApi "github.com/mittwald/go-powerdns"
	"github.com/mittwald/go-powerdns/apis/search"
	"github.com/mittwald/go-powerdns/apis/zones"
	"github.com/mixanemca/pdns-api/internal/infrastructure/errors"
	"github.com/mixanemca/pdns-api/internal/infrastructure/network"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

type PTR struct {
	logger *logrus.Logger
	auth   pdnsApi.Client
}

func NewPTR(logger *logrus.Logger, auth pdnsApi.Client) *PTR {
	return &PTR{logger: logger, auth: auth}
}

// AddPTR check exists PTR record by name, remove old PTR and add new
func (s *PTR) AddPTR(ctx context.Context, serverID string, zoneID string, rrset zones.ResourceRecordSet) error {
	// Use A and AAAA records only
	if !(rrset.Type == "A" || rrset.Type == "AAAA") {
		return nil
	}

	err := s.DelPTR(ctx, serverID, zoneID, rrset)
	if err != nil {
		return err
	}

	for _, record := range rrset.Records {
		reverse, err := dns.ReverseAddr(record.Content)
		if err != nil {
			return errors.Wrapf(err, "failed to get reverse address for %s", record.Content)
		}
		ptrRRSet := zones.ResourceRecordSet{
			Name:       reverse,
			Type:       "PTR",
			TTL:        rrset.TTL,
			ChangeType: zones.ChangeTypeReplace,
			Records: []zones.Record{
				{
					Content:  rrset.Name,
					Disabled: false,
				},
			},
		}
		err = s.auth.Zones().AddRecordSetToZone(ctx, serverID, "10.in-addr.arpa", ptrRRSet)
		if err != nil {
			return errors.Wrapf(err, "failed to update reverse zone %s", LocalReverseZone)
		}
		s.logger.Infof("Reverse record %s was added with content %s", reverse, rrset.Name)
	}

	return nil
}

// DelPTR removes PTR record
func (s *PTR) DelPTR(ctx context.Context, serverID string, zoneID string, rrset zones.ResourceRecordSet) error {
	// Use A and AAAA records only
	if !(rrset.Type == "A" || rrset.Type == "AAAA") {
		return nil
	}

	results, err := s.auth.Search().Search(ctx, serverID, network.DeCanonicalize(rrset.Name), 10, search.ObjectTypeRecord)
	if err != nil {
		return errors.Wrapf(err, "searching for zone %s and RR %s", zoneID, rrset.Name)
	}
	for _, result := range results {
		if strings.ToUpper(result.Type) == "PTR" {
			s.logger.Infof("Remove old PTR %s", result.Name)
			err = s.auth.Zones().RemoveRecordSetFromZone(ctx, serverID, "10.in-addr.arpa.", result.Name, result.Type)
			if err != nil {
				return errors.Wrapf(err, "deleting RR %s from reverse zone %s", result.Name, LocalReverseZone)
			}
		}
	}
	// TODO: flush cache for 10.in-addr.arpa. zone

	return nil
}
