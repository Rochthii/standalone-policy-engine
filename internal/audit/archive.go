package audit

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

const archiveFormatVersion = 1

// ArchiveSink persists sealed audit entries to an independently administered archive.
type ArchiveSink interface {
	Archive(context.Context, []*LogEntry) error
}

// ArchivingBatchWriter commits operational storage before submitting the same batch to the archive.
// A failed archive write returns an error so the existing encrypted spill/replay flow retries it.
type ArchivingBatchWriter struct {
	durable BatchWriter
	archive ArchiveSink
}

func NewArchivingBatchWriter(durable BatchWriter, archive ArchiveSink) (*ArchivingBatchWriter, error) {
	if durable == nil || archive == nil {
		return nil, errors.New("durable writer and audit archive are required")
	}
	return &ArchivingBatchWriter{durable: durable, archive: archive}, nil
}

func (w *ArchivingBatchWriter) InsertAuditLogsBatch(ctx context.Context, logs []*LogEntry) error {
	if err := w.durable.InsertAuditLogsBatch(ctx, logs); err != nil {
		return err
	}
	if err := w.archive.Archive(ctx, logs); err != nil {
		return fmt.Errorf("archive committed audit entries: %w", err)
	}
	return nil
}

// S3ArchiveConfig limits archive delivery to a pre-provisioned Object Lock bucket.
type S3ArchiveConfig struct {
	Bucket              string
	Prefix              string
	Region              string
	ExpectedBucketOwner string
}

type s3PutObjectAPI interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// S3ArchiveSink writes deterministic, create-only NDJSON segments. Object Lock is enforced by the bucket's IaC default retention.
type S3ArchiveSink struct {
	client s3PutObjectAPI
	config S3ArchiveConfig
}

func NewS3ArchiveSink(ctx context.Context, cfg S3ArchiveConfig) (*S3ArchiveSink, error) {
	if err := validateS3ArchiveConfig(cfg); err != nil {
		return nil, err
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("load AWS archive configuration: %w", err)
	}
	return newS3ArchiveSink(s3.NewFromConfig(awsCfg), cfg)
}

func newS3ArchiveSink(client s3PutObjectAPI, cfg S3ArchiveConfig) (*S3ArchiveSink, error) {
	if client == nil {
		return nil, errors.New("S3 archive client is required")
	}
	if err := validateS3ArchiveConfig(cfg); err != nil {
		return nil, err
	}
	return &S3ArchiveSink{client: client, config: cfg}, nil
}

func (s *S3ArchiveSink) Archive(ctx context.Context, entries []*LogEntry) error {
	for _, segment := range archiveSegments(s.config.Prefix, entries) {
		if err := segment.err; err != nil {
			return err
		}
		_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:              aws.String(s.config.Bucket),
			Key:                 aws.String(segment.key),
			Body:                bytes.NewReader(segment.payload),
			ContentType:         aws.String("application/x-ndjson"),
			ContentMD5:          aws.String(segment.contentMD5),
			ChecksumSHA256:      aws.String(segment.checksumSHA256),
			ExpectedBucketOwner: aws.String(s.config.ExpectedBucketOwner),
			IfNoneMatch:         aws.String("*"),
			Metadata: map[string]string{
				"audit-format-version": fmt.Sprintf("%d", archiveFormatVersion),
				"audit-entry-count":    fmt.Sprintf("%d", segment.entryCount),
				"audit-sha256":         segment.sha256Hex,
			},
		})
		if err != nil && !isArchiveAlreadyPresent(err) {
			return fmt.Errorf("put immutable audit segment %s: %w", segment.key, err)
		}
	}
	return nil
}

type archiveSegment struct {
	key            string
	payload        []byte
	contentMD5     string
	checksumSHA256 string
	sha256Hex      string
	entryCount     int
	err            error
}

func archiveSegments(prefix string, entries []*LogEntry) []archiveSegment {
	byTenant := make(map[string][]*LogEntry)
	for _, entry := range entries {
		if err := validateArchiveEntry(entry); err != nil {
			return []archiveSegment{{err: err}}
		}
		byTenant[entry.TenantID] = append(byTenant[entry.TenantID], entry)
	}
	tenants := make([]string, 0, len(byTenant))
	for tenantID := range byTenant {
		tenants = append(tenants, tenantID)
	}
	sort.Strings(tenants)
	segments := make([]archiveSegment, 0, len(tenants))
	for _, tenantID := range tenants {
		segments = append(segments, newArchiveSegment(prefix, tenantID, byTenant[tenantID]))
	}
	return segments
}

func newArchiveSegment(prefix, tenantID string, entries []*LogEntry) archiveSegment {
	var payload bytes.Buffer
	for _, entry := range entries {
		encoded, err := json.Marshal(entry)
		if err != nil {
			return archiveSegment{err: fmt.Errorf("encode audit archive entry: %w", err)}
		}
		payload.Write(encoded)
		payload.WriteByte('\n')
	}
	data := payload.Bytes()
	sha := sha256.Sum256(data)
	md5Sum := md5.Sum(data)
	date := time.Unix(0, entries[0].Timestamp).UTC().Format("2006-01-02")
	return archiveSegment{
		key:            fmt.Sprintf("%s/tenant=%s/date=%s/%s.ndjson", prefix, tenantID, date, hex.EncodeToString(sha[:])),
		payload:        data,
		contentMD5:     base64.StdEncoding.EncodeToString(md5Sum[:]),
		checksumSHA256: base64.StdEncoding.EncodeToString(sha[:]),
		sha256Hex:      hex.EncodeToString(sha[:]),
		entryCount:     len(entries),
	}
}

func validateArchiveEntry(entry *LogEntry) error {
	if entry == nil || entry.AuditID == "" || entry.TenantID == "" || entry.Timestamp <= 0 {
		return errors.New("archive entry requires audit ID, tenant ID and timestamp")
	}
	if strings.ContainsAny(entry.TenantID, "/\\") {
		return errors.New("archive tenant ID must not contain a path separator")
	}
	if !entry.IsEncrypted || entry.Subject != "" || entry.Action != "" || entry.Resource != "" || entry.Context != nil {
		return errors.New("archive refuses unsealed audit entry")
	}
	return nil
}

func validateS3ArchiveConfig(cfg S3ArchiveConfig) error {
	if strings.TrimSpace(cfg.Bucket) == "" || strings.TrimSpace(cfg.Region) == "" || strings.TrimSpace(cfg.ExpectedBucketOwner) == "" {
		return errors.New("S3 archive bucket, region and expected owner are required")
	}
	if strings.TrimSpace(cfg.Prefix) == "" || strings.Trim(cfg.Prefix, "/") != cfg.Prefix || strings.Contains(cfg.Prefix, "..") {
		return errors.New("S3 archive prefix must be non-empty and safe")
	}
	if len(cfg.ExpectedBucketOwner) != 12 {
		return errors.New("S3 archive expected owner must be a 12-digit AWS account ID")
	}
	for _, char := range cfg.ExpectedBucketOwner {
		if char < '0' || char > '9' {
			return errors.New("S3 archive expected owner must be a 12-digit AWS account ID")
		}
	}
	return nil
}

func isArchiveAlreadyPresent(err error) bool {
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == "PreconditionFailed"
}
