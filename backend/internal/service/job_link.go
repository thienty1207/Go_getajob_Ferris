package service

import (
	"context"
	"errors"
	"net"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrInvalidJobLinkURL      = errors.New("invalid job link URL")
	ErrInvalidJobLinkApproval = errors.New("invalid job link approval")
	ErrInvalidJobLinkID       = errors.New("invalid job link ID")
	ErrInvalidJobLinkStatus   = errors.New("invalid job link status")
)

type JobLinkInput struct {
	URL        string
	ApprovedBy string
}

type JobLinkService struct {
	repository repository.JobLinkRepository
}

func NewJobLinkService(jobLinkRepository repository.JobLinkRepository) *JobLinkService {
	return &JobLinkService{repository: jobLinkRepository}
}

func (s *JobLinkService) ListJobLinks(ctx context.Context, page, pageSize int) (model.JobLinkPage, error) {
	return s.repository.ListJobLinks(ctx, page, pageSize)
}

func (s *JobLinkService) Create(ctx context.Context, input JobLinkInput) (model.JobLink, error) {
	normalizedURL, displayName, err := normalizeJobLinkURL(input.URL)
	if err != nil {
		return model.JobLink{}, err
	}
	approvedBy, err := normalizeApprovalActor(input.ApprovedBy)
	if err != nil {
		return model.JobLink{}, err
	}
	now := time.Now().UTC()
	id := uuid.New()
	approvedByPointer := &approvedBy
	approvedAtPointer := &now
	return s.repository.CreateJobLink(ctx, repository.JobLinkWrite{
		ID:             id,
		SourceKey:      "source-" + id.String(),
		DisplayName:    displayName,
		BaseURL:        normalizedURL,
		SourceType:     "EXPLICIT_PERMISSION",
		ApprovalStatus: "ACTIVE",
		ApprovedAt:     approvedAtPointer,
		ApprovedBy:     approvedByPointer,
	})
}

func (s *JobLinkService) Update(ctx context.Context, id uuid.UUID, input JobLinkInput) (model.JobLink, error) {
	if id == uuid.Nil {
		return model.JobLink{}, ErrInvalidJobLinkID
	}
	normalizedURL, displayName, err := normalizeJobLinkURL(input.URL)
	if err != nil {
		return model.JobLink{}, err
	}
	approvedBy, err := normalizeApprovalActor(input.ApprovedBy)
	if err != nil {
		return model.JobLink{}, err
	}
	now := time.Now().UTC()
	approvedByPointer := &approvedBy
	approvedAtPointer := &now
	return s.repository.UpdateJobLink(ctx, repository.JobLinkWrite{
		ID:          id,
		DisplayName: displayName,
		BaseURL:     normalizedURL,
		ApprovedAt:  approvedAtPointer,
		ApprovedBy:  approvedByPointer,
	})
}

func (s *JobLinkService) SetStatus(ctx context.Context, id uuid.UUID, status string) error {
	if id == uuid.Nil {
		return ErrInvalidJobLinkID
	}
	status = strings.ToUpper(strings.TrimSpace(status))
	if status != "ACTIVE" && status != "DISABLED" {
		return ErrInvalidJobLinkStatus
	}
	return s.repository.SetJobLinkStatus(ctx, id, status)
}

func (s *JobLinkService) Delete(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return ErrInvalidJobLinkID
	}
	return s.repository.DeleteJobLink(ctx, id)
}

func normalizeApprovalActor(raw string) (string, error) {
	actor := strings.TrimSpace(raw)
	if actor == "" || len(actor) > 320 || strings.ContainsAny(actor, "\r\n") {
		return "", ErrInvalidJobLinkApproval
	}
	return actor, nil
}

func normalizeJobLinkURL(raw string) (string, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.ContainsAny(trimmed, "\r\n") || len(trimmed) > 2000 {
		return "", "", ErrInvalidJobLinkURL
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.User != nil || parsed.Host == "" {
		return "", "", ErrInvalidJobLinkURL
	}
	if !isValidExplicitPort(parsed) || hasAmbiguousEncodedPath(parsed) {
		return "", "", ErrInvalidJobLinkURL
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", ErrInvalidJobLinkURL
	}
	hostname := canonicalSourceHostname(parsed.Hostname())
	if !isPublicSourceHostname(hostname) {
		return "", "", ErrInvalidJobLinkURL
	}
	parsed.Host = canonicalJobLinkHost(parsed.Scheme, hostname, canonicalExplicitPort(parsed))
	parsed.Fragment = ""
	parsed.RawFragment = ""
	parsed.Path = normalizeSourcePath(parsed.Path)
	parsed.RawPath = ""
	displayName := parsed.Hostname()
	if displayName == "" {
		return "", "", ErrInvalidJobLinkURL
	}
	return parsed.String(), displayName, nil
}

func canonicalSourceHostname(hostname string) string {
	hostname = strings.TrimRight(strings.ToLower(hostname), ".")
	if ip := net.ParseIP(hostname); ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4.String()
		}
		return ip.String()
	}
	return hostname
}

func canonicalJobLinkHost(scheme, hostname, port string) string {
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	if strings.Contains(hostname, ":") {
		hostname = "[" + hostname + "]"
	}
	if port == "" {
		return hostname
	}
	return hostname + ":" + port
}

func canonicalExplicitPort(parsed *url.URL) string {
	rawPort := parsed.Port()
	if rawPort == "" {
		return ""
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		return rawPort
	}
	return strconv.Itoa(port)
}

func isValidExplicitPort(parsed *url.URL) bool {
	if parsed.Port() == "" {
		return !hasExplicitPort(parsed)
	}
	value, err := strconv.Atoi(parsed.Port())
	return err == nil && value >= 1 && value <= 65535
}

func hasExplicitPort(parsed *url.URL) bool {
	host := parsed.Host
	if strings.HasPrefix(host, "[") {
		closingBracket := strings.LastIndex(host, "]")
		return closingBracket >= 0 && len(host) > closingBracket+1
	}
	return strings.Count(host, ":") == 1
}

func hasAmbiguousEncodedPath(parsed *url.URL) bool {
	escapedPath := strings.ToLower(parsed.EscapedPath())
	for _, escape := range []string{"%2e", "%2f", "%5c"} {
		if strings.Contains(escapedPath, escape) {
			return true
		}
	}
	return false
}

func normalizeSourcePath(rawPath string) string {
	cleaned := path.Clean("/" + strings.TrimSpace(rawPath))
	if cleaned == "." || cleaned == "/" {
		return "/"
	}
	return strings.TrimSuffix(cleaned, "/") + "/"
}

func isPublicSourceHostname(hostname string) bool {
	if hostname == "" || strings.Contains(hostname, "%") || hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") || strings.HasSuffix(hostname, ".local") || strings.HasSuffix(hostname, ".internal") {
		return false
	}
	if ip := net.ParseIP(hostname); ip != nil {
		return isPublicSourceIP(ip)
	}
	return true
}

var sourceIPv4SpecialRanges = mustParseSourceCIDRs([]string{
	"0.0.0.0/8",
	"100.64.0.0/10",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"192.31.196.0/24",
	"192.52.193.0/24",
	"192.88.99.0/24",
	"192.175.48.0/24",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"224.0.0.0/4",
	"240.0.0.0/4",
})

var sourceIPv6SpecialRanges = mustParseSourceCIDRs([]string{
	"64:ff9b::/96",
	"64:ff9b:1::/48",
	"2001::/23",
	"2001:2::/48",
	"2001:10::/28",
	"2001:20::/28",
	"2001:db8::/32",
	"2002::/16",
	"3000::/4",
	"3fff::/20",
	"ff00::/8",
})

var sourceIPv6GlobalRange = mustParseSourceCIDRs([]string{"2000::/3"})[0]

func isPublicSourceIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return false
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return !ipv4.IsPrivate() && !containsSourceCIDR(sourceIPv4SpecialRanges, ipv4)
	}
	if !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return false
	}
	return sourceIPv6GlobalRange.Contains(ip) && !containsSourceCIDR(sourceIPv6SpecialRanges, ip)
}

func mustParseSourceCIDRs(values []string) []*net.IPNet {
	networks := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			panic("invalid source CIDR: " + value)
		}
		networks = append(networks, network)
	}
	return networks
}

func containsSourceCIDR(networks []*net.IPNet, ip net.IP) bool {
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

var _ repository.JobLinkRepository = (*repository.PostgresJobLinkRepository)(nil)
