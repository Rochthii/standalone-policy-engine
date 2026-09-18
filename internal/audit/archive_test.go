package audit

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	"standalone-policy-engine/internal/security"
)

type recordingS3PutClient struct {
	requests []*s3.PutObjectInput
	err      error
}

type recordingArchiveSink struct {
	entries []*LogEntry
	err     error
}

func (s *recordingArchiveSink) Archive(_ context.Context, entries []*LogEntry) error {
	s.entries = append(s.entries, entries...)
	return s.err
}

func (c *recordingS3PutClient) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	copyInput := *input
	if input.Body != nil {
		data, err := io.ReadAll(input.Body)
		if err != nil {
			return nil, err
		}
		copyInput.Body = bytes.NewReader(data)
	}
	c.requests = append(c.requests, &copyInput)
	return &s3.PutObjectOutput{}, c.err
}

func sealedArchiveEntry(t *testing.T, tenantID, auditID string) *LogEntry {
	t.Helper()
	crypto, err := security.NewEnvelopeCryptoWithKeyring("archive", map[string]string{"archive": testNewAuditKEK})
	if err != nil {
		t.Fatal(err)
	}
	entry := &LogEntry{
		AuditID: auditID, Timestamp: time.Date(2029, time.January, 2, 3, 4, 5, 0, time.UTC).UnixNano(),
		TenantID: tenantID, Subject: "user:alice", Action: "WRITE", Resource: "purchase:3", Decision: "ALLOW",
	}
	if err := sealAuditEntry(crypto, entry); err != nil {
		t.Fatal(err)
	}
	return entry
}

func TestS3ArchiveWritesCreateOnlyEncryptedTenantSegments(t *testing.T) {
	client := &recordingS3PutClient{}
	sink, err := newS3ArchiveSink(client, S3ArchiveConfig{Bucket: "archive", Prefix: "pdp-audit", Region: "ap-southeast-1", ExpectedBucketOwner: "123456789012"})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Archive(context.Background(), []*LogEntry{
		sealedArchiveEntry(t, "tenant-b", "audit-b"),
		sealedArchiveEntry(t, "tenant-a", "audit-a"),
	}); err != nil {
		t.Fatal(err)
	}
	if len(client.requests) != 2 {
		t.Fatalf("expected one immutable segment per tenant, got %d", len(client.requests))
	}
	for _, request := range client.requests {
		if request.IfNoneMatch == nil || *request.IfNoneMatch != "*" || request.ContentMD5 == nil || request.ChecksumSHA256 == nil {
			t.Fatalf("archive request must be create-only and integrity checked: %#v", request)
		}
		if request.ExpectedBucketOwner == nil || *request.ExpectedBucketOwner != "123456789012" {
			t.Fatalf("archive request must bind the expected bucket owner: %#v", request)
		}
		payload, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(payload, []byte("user:alice")) || bytes.Contains(payload, []byte("purchase:3")) {
			t.Fatalf("archive payload leaked plaintext audit fields: %s", payload)
		}
	}
	if *client.requests[0].Key >= *client.requests[1].Key {
		t.Fatalf("tenant archive order must be stable: %q >= %q", *client.requests[0].Key, *client.requests[1].Key)
	}
}

func TestArchivingBatchWriterCommitsPostgresBeforeArchive(t *testing.T) {
	durable := &recordingBatchWriter{written: make(chan struct{}, 1)}
	archive := &recordingArchiveSink{}
	writer, err := NewArchivingBatchWriter(durable, archive)
	if err != nil {
		t.Fatal(err)
	}
	entry := sealedArchiveEntry(t, "tenant-a", "audit-a")
	if err := writer.InsertAuditLogsBatch(context.Background(), []*LogEntry{entry}); err != nil {
		t.Fatal(err)
	}
	if len(durable.entries) != 1 || len(archive.entries) != 1 {
		t.Fatalf("durable and archive writes must both receive the entry: durable=%d archive=%d", len(durable.entries), len(archive.entries))
	}

	archive.err = errors.New("S3 unavailable")
	if err := writer.InsertAuditLogsBatch(context.Background(), []*LogEntry{entry}); err == nil {
		t.Fatal("archive failure must reach the batch logger for spill/replay")
	}
	if len(durable.entries) != 2 {
		t.Fatal("durable PostgreSQL write must complete before archive failure")
	}
}

func TestS3ArchiveTreatsExistingImmutableSegmentAsReplaySuccess(t *testing.T) {
	client := &recordingS3PutClient{err: archivePreconditionError{}}
	sink, err := newS3ArchiveSink(client, S3ArchiveConfig{Bucket: "archive", Prefix: "pdp-audit", Region: "ap-southeast-1", ExpectedBucketOwner: "123456789012"})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Archive(context.Background(), []*LogEntry{sealedArchiveEntry(t, "tenant-a", "audit-a")}); err != nil {
		t.Fatalf("replay of an existing immutable segment must succeed: %v", err)
	}
}

func TestS3ArchiveRejectsPlaintextEntry(t *testing.T) {
	client := &recordingS3PutClient{}
	sink, err := newS3ArchiveSink(client, S3ArchiveConfig{Bucket: "archive", Prefix: "pdp-audit", Region: "ap-southeast-1", ExpectedBucketOwner: "123456789012"})
	if err != nil {
		t.Fatal(err)
	}
	err = sink.Archive(context.Background(), []*LogEntry{{AuditID: "audit-a", TenantID: "tenant-a", Timestamp: time.Now().UnixNano(), Subject: "plaintext"}})
	if err == nil {
		t.Fatalf("expected plaintext archive entry rejection, got %v", err)
	}
}

type archivePreconditionError struct{}

func (archivePreconditionError) Error() string     { return "precondition failed" }
func (archivePreconditionError) ErrorCode() string { return "PreconditionFailed" }
func (archivePreconditionError) ErrorMessage() string {
	return "immutable segment already exists"
}
func (archivePreconditionError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }
