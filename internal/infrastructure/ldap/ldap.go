package ldap

import (
	"fmt"

	"github.com/go-ldap/ldap"
	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/mixanemca/pdns-api/internal/infrastructure/errors"
	log "github.com/mixanemca/pdns-api/internal/infrastructure/logger"
	"github.com/mixanemca/pdns-api/internal/infrastructure/network"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	searchFilter string = `(|(|(&(memberof=cn=%s,ou=%s,ou=%s,ou=dnsaas,ou=groups,dc=avito,dc=ru)(uid=%s))(&(memberof=cn=%s,ou=%s,ou=dnsaas,ou=groups,dc=avito,dc=ru)(uid=%s)))(&(memberof=cn=%s,ou=dnsaas,ou=groups,dc=avito,dc=ru)(uid=%s)))`
)

type LDAPZoneAdder interface {
	LDAPAddZone(zoneType, zone string) error
}

type LDAPZoneDeleter interface {
	LDAPDelZone(zoneType, zone string) error
}

type ldapService struct {
	logger     *logrus.Logger
	config     config.Config
	ldapClient *ldap.Conn
}

func NewLDAPService(logger *logrus.Logger, config config.Config) (*ldapService, error) {
	var err error
	s := &ldapService{logger: logger, config: config}
	if viper.GetBool("ldap.enabled") {
		err = s.LDAPInit()
	}
	return s, err
}

// LDAPInit connecting to LDAP server and performs a bind with the given username and password from config.
func (s *ldapService) LDAPInit() (err error) {
	// connects to the given ldap URL vie TCP using tls.Dial or net.Dial if ldaps:// or ldap:// specified as protocol
	s.logger.WithFields(logrus.Fields{
		"action": log.ActionLDAPConnect,
	}).Debugf("Connect to LDAP server %s", s.config.LDAP.URL)

	s.ldapClient, err = ldap.DialURL(s.config.LDAP.URL)
	if err != nil {
		return err
	}

	// performs a bind with the given username and password
	s.logger.WithFields(logrus.Fields{
		"action": log.ActionLDAPConnect,
	}).Debugf("LDAP bind with username %s", s.config.LDAP.User)

	err = s.ldapClient.Bind(fmt.Sprintf("uid=%s,%s", s.config.LDAP.User, s.config.LDAP.BaseDN), s.config.LDAP.Password)
	if err != nil {
		return err
	}

	return nil
}

// AuthorizeViaLDAP doing search request to LDAP server.
// Return bool value for search result or error.
func (s *ldapService) AuthorizeViaLDAP(cnType, zoneType, zone, username string) (bool, error) {
	// Reconnect to LDAP service is connection is clossed
	if s.ldapClient.IsClosing() {
		if err := s.LDAPInit(); err != nil {
			return false, errors.Wrapf(err, "authorizing %s for %s %s (zone %s)", username, cnType, zoneType, zone)
		}
	}

	// Makes a new search request
	searchRequest := ldap.NewSearchRequest(
		s.config.LDAP.SearchBase,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf(searchFilter,
			cnType, zone, zoneType, username,
			cnType, zoneType, username,
			cnType, username,
		),
		[]string{"uid"},
		nil,
	)

	sr, err := s.ldapClient.Search(searchRequest)
	if err != nil {
		return false, errors.Wrapf(err, "authorizing %s for %s %s (zone %s)", username, cnType, zoneType, zone)
	}

	// Return false (unauthorized) if ldap.SearchResult.Entries not found
	if len(sr.Entries) < 1 {
		return false, nil
	}

	// OK
	return true, nil
}

// LDAPAddZone creates LDAP Organizational Unit with zone name
// and adds two Common Names (CN) for replace and delete checks.
func (s *ldapService) LDAPAddZone(zoneType, zone string) error {
	// Reconnect to LDAP service is connection is clossed
	if s.ldapClient.IsClosing() {
		if err := s.LDAPInit(); err != nil {
			return errors.Wrapf(err, "adding %s zone to %s", zone, zoneType)
		}
	}

	dn := fmt.Sprintf("ou=%s,ou=%s,ou=dnsaas,ou=groups,%s", zone, zoneType, s.config.LDAP.SearchBase)
	s.logger.WithFields(logrus.Fields{
		"action": log.ActionLDAPAddZone,
	}).Debugf("DN: %s", dn)

	// Makes a new add request
	addOUReq := ldap.NewAddRequest(
		dn,
		[]ldap.Control{},
	)
	addOUReq.Attribute("objectClass", []string{"organizationalUnit", "top"})

	// Do request
	s.logger.WithFields(logrus.Fields{
		"action": log.ActionLDAPAddZone,
	}).Debugf("Add %s %s to LDAP", zoneType, network.DeCanonicalize(zone))
	if err := s.ldapClient.Add(addOUReq); err != nil {
		return errors.Wrapf(err, "adding %s zone to %s", zone, zoneType)
	}

	// Add CN's to new zone
	if err := s.LDAPAddCN(zoneType, zone, CNTypeReplace, CNTypeDelete); err != nil {
		// Cleanup
		_ = s.LDAPDelZone(zoneType, zone)
		return errors.Wrapf(err, "adding %s zone to %s", zone, zoneType)
	}

	return nil
}

// LDAPDelZone deletes LDAP Organizational Unit with zone name
// add deletes Common Names (CN) for replace and delete checks
func (s *ldapService) LDAPDelZone(zoneType, zone string) error {
	// Reconnect to LDAP service is connection is clossed
	if s.ldapClient.IsClosing() {
		if err := s.LDAPInit(); err != nil {
			return errors.Wrapf(err, "remove %s zone from %s", zone, zoneType)
		}
	}

	// Del CN's from zone
	if err := s.LDAPDelCN(zoneType, zone, CNTypeReplace, CNTypeDelete); err != nil {
		return errors.Wrapf(err, "remove %s zone from %s", zone, zoneType)
	}

	dn := fmt.Sprintf("ou=%s,ou=%s,ou=dnsaas,ou=groups,%s", zone, zoneType, s.config.LDAP.SearchBase)
	s.logger.WithFields(logrus.Fields{
		"action": log.ActionLDAPDelZone,
	}).Debugf("DN: %s", dn)

	// Makes a new delete request
	delReq := ldap.NewDelRequest(
		dn,
		[]ldap.Control{},
	)
	// Do request
	s.logger.WithFields(logrus.Fields{
		"action": log.ActionLDAPDelZone,
	}).Debugf("Delete %s %s from LDAP", zoneType, zone)
	if err := s.ldapClient.Del(delReq); err != nil {
		return errors.Wrapf(err, "remove %s zone from %s", zone, zoneType)
	}

	return nil
}

// LDAPAddCN added groupOfNames (delete, replace) to LDAP.
func (s *ldapService) LDAPAddCN(zoneType, zone string, cnTypes ...string) error {
	for _, t := range cnTypes {
		if err := s.cnAdd(zoneType, zone, t); err != nil {
			return errors.Wrapf(err, "adding CN %s", t)
		}
	}

	return nil
}

// LDAPDelCN added groupOfNames (delete, replace) to LDAP.
func (s *ldapService) LDAPDelCN(zoneType, zone string, cnTypes ...string) error {
	for _, t := range cnTypes {
		if err := s.cnDel(zoneType, zone, t); err != nil {
			return errors.Wrapf(err, "removing CN %s", t)
		}
	}

	return nil
}

func (s *ldapService) cnAdd(zoneType, zone, cnType string) error {
	// Makes a new add request.
	addCNReq := ldap.NewAddRequest(
		fmt.Sprintf("cn=%s,ou=%s,ou=%s,ou=dnsaas,ou=groups,%s", cnType, zone, zoneType, s.config.LDAP.SearchBase),
		[]ldap.Control{},
	)
	addCNReq.Attribute("objectClass", []string{"groupOfNames", "top"})
	// Creating CN required one or more members
	// Add user from config by default
	addCNReq.Attribute("member", []string{
		fmt.Sprintf("uid=%s,%s", s.config.LDAP.User, s.config.LDAP.BaseDN),
	})

	s.logger.WithFields(logrus.Fields{
		"action": log.ActionLDAPAddCN,
	}).Debugf("Add CN %s to %s %s", cnType, zoneType, zone)
	// Do request
	if err := s.ldapClient.Add(addCNReq); err != nil {
		if e, ok := err.(*ldap.Error); ok {
			switch e.ResultCode {
			case ldap.LDAPResultNoSuchObject:
				err = errors.NotFound.Wrapf(err, "can't add CN %s", cnType)
			case ldap.LDAPResultEntryAlreadyExists:
				err = errors.Conflict.Wrapf(err, "can't add CN %s", cnType)
			}
		}

		return err
	}

	return nil
}

func (s *ldapService) cnDel(zoneType, zone, cnType string) error {
	// Makes a new delete request
	delCNReq := ldap.NewDelRequest(
		fmt.Sprintf("cn=%s,ou=%s,ou=%s,ou=dnsaas,ou=groups,%s", cnType, zone, zoneType, s.config.LDAP.SearchBase),
		[]ldap.Control{},
	)

	s.logger.WithFields(logrus.Fields{
		"action": log.ActionLDAPDelCN,
	}).Debugf("Delete CN %s from %s %s", cnType, zone, zoneType)
	// Do request
	if err := s.ldapClient.Del(delCNReq); err != nil {
		if e, ok := err.(*ldap.Error); ok {
			if e.ResultCode == ldap.LDAPResultNoSuchObject {
				err = errors.NotFound.Wrapf(err, "can't remove CN %s", cnType)
			}
		}

		return err
	}

	return nil
}
