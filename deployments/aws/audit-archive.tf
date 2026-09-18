terraform {
  required_version = ">= 1.6.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

provider "aws" {
  region = var.region
}

variable "region" {
  type = string
}

variable "archive_bucket_name" {
  type = string
}

variable "evidence_bucket_name" {
  type = string
}

variable "publisher_role_arn" {
  type = string
}

variable "archive_prefix" {
  type    = string
  default = "pdp-audit"
}

variable "archive_retention_years" {
  type    = number
  default = 7

  validation {
    condition     = var.archive_retention_years == 7
    error_message = "The approved thesis archive retention is seven years."
  }
}

variable "evidence_retention_years" {
  type    = number
  default = 9

  validation {
    condition     = var.evidence_retention_years == 9
    error_message = "The approved thesis evidence retention is nine years."
  }
}

data "aws_caller_identity" "current" {}

data "aws_partition" "current" {}

locals {
  cloudtrail_arn = "arn:${data.aws_partition.current.partition}:cloudtrail:${var.region}:${data.aws_caller_identity.current.account_id}:trail/pdp-audit-archive"
}

resource "aws_kms_key" "archive" {
  description             = "PDP immutable audit archive encryption"
  enable_key_rotation     = true
  deletion_window_in_days = 30
}

data "aws_iam_policy_document" "archive_kms" {
  statement {
    sid       = "AllowArchiveAccountAdministration"
    effect    = "Allow"
    actions   = ["kms:*"]
    resources = ["*"]

    principals {
      type        = "AWS"
      identifiers = ["arn:${data.aws_partition.current.partition}:iam::${data.aws_caller_identity.current.account_id}:root"]
    }
  }

  statement {
    sid       = "AllowPublisherEncryptionOnly"
    effect    = "Allow"
    actions   = ["kms:Encrypt", "kms:GenerateDataKey", "kms:DescribeKey"]
    resources = ["*"]

    principals {
      type        = "AWS"
      identifiers = [var.publisher_role_arn]
    }
  }

  statement {
    sid       = "AllowCloudTrailLogEncryption"
    effect    = "Allow"
    actions   = ["kms:GenerateDataKey*"]
    resources = ["*"]

    principals {
      type        = "Service"
      identifiers = ["cloudtrail.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "aws:SourceArn"
      values   = [local.cloudtrail_arn]
    }
  }
}

resource "aws_kms_key_policy" "archive" {
  key_id = aws_kms_key.archive.key_id
  policy = data.aws_iam_policy_document.archive_kms.json
}

resource "aws_s3_bucket" "archive" {
  bucket              = var.archive_bucket_name
  object_lock_enabled = true
}

resource "aws_s3_bucket_versioning" "archive" {
  bucket = aws_s3_bucket.archive.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_object_lock_configuration" "archive" {
  bucket = aws_s3_bucket.archive.id

  rule {
    default_retention {
      mode  = "COMPLIANCE"
      years = var.archive_retention_years
    }
  }
}

resource "aws_s3_bucket_ownership_controls" "archive" {
  bucket = aws_s3_bucket.archive.id

  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

resource "aws_s3_bucket_public_access_block" "archive" {
  bucket                  = aws_s3_bucket.archive.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "archive" {
  bucket = aws_s3_bucket.archive.id

  rule {
    apply_server_side_encryption_by_default {
      kms_master_key_id = aws_kms_key.archive.arn
      sse_algorithm     = "aws:kms"
    }
    bucket_key_enabled = true
  }
}

resource "aws_s3_bucket" "evidence" {
  bucket              = var.evidence_bucket_name
  object_lock_enabled = true
}

resource "aws_s3_bucket_versioning" "evidence" {
  bucket = aws_s3_bucket.evidence.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_object_lock_configuration" "evidence" {
  bucket = aws_s3_bucket.evidence.id

  rule {
    default_retention {
      mode  = "COMPLIANCE"
      years = var.evidence_retention_years
    }
  }
}

resource "aws_s3_bucket_ownership_controls" "evidence" {
  bucket = aws_s3_bucket.evidence.id

  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

resource "aws_s3_bucket_public_access_block" "evidence" {
  bucket                  = aws_s3_bucket.evidence.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "evidence" {
  bucket = aws_s3_bucket.evidence.id

  rule {
    apply_server_side_encryption_by_default {
      kms_master_key_id = aws_kms_key.archive.arn
      sse_algorithm     = "aws:kms"
    }
    bucket_key_enabled = true
  }
}

resource "aws_cloudtrail" "audit_archive" {
  name                          = "pdp-audit-archive"
  s3_bucket_name                = aws_s3_bucket.evidence.id
  enable_log_file_validation    = true
  include_global_service_events = true
  is_multi_region_trail         = true
  depends_on = [
    aws_kms_key_policy.archive,
    aws_s3_bucket_policy.evidence,
    aws_s3_bucket_server_side_encryption_configuration.evidence,
  ]

  event_selector {
    read_write_type           = "All"
    include_management_events = true

    data_resource {
      type = "AWS::S3::Object"
      values = [
        "${aws_s3_bucket.archive.arn}/",
        "${aws_s3_bucket.evidence.arn}/",
      ]
    }
  }
}

data "aws_iam_policy_document" "archive_bucket" {
  statement {
    sid     = "AllowPublisherCreateOnly"
    effect  = "Allow"
    actions = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.archive.arn}/${var.archive_prefix}/*"]

    principals {
      type        = "AWS"
      identifiers = [var.publisher_role_arn]
    }
  }

  statement {
    sid    = "DenyInsecureTransport"
    effect = "Deny"
    actions = ["s3:*"]
    resources = [
      aws_s3_bucket.archive.arn,
      "${aws_s3_bucket.archive.arn}/*",
    ]

    principals {
      type        = "*"
      identifiers = ["*"]
    }

    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
}

resource "aws_s3_bucket_policy" "archive" {
  bucket = aws_s3_bucket.archive.id
  policy = data.aws_iam_policy_document.archive_bucket.json
}

data "aws_iam_policy_document" "evidence_bucket" {
  statement {
    sid     = "AllowCloudTrailAclCheck"
    effect  = "Allow"
    actions = ["s3:GetBucketAcl"]
    resources = [aws_s3_bucket.evidence.arn]

    principals {
      type        = "Service"
      identifiers = ["cloudtrail.amazonaws.com"]
    }
  }

  statement {
    sid     = "AllowCloudTrailWrite"
    effect  = "Allow"
    actions = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.evidence.arn}/AWSLogs/${data.aws_caller_identity.current.account_id}/*"]

    principals {
      type        = "Service"
      identifiers = ["cloudtrail.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "s3:x-amz-acl"
      values   = ["bucket-owner-full-control"]
    }
  }

  statement {
    sid    = "DenyInsecureTransport"
    effect = "Deny"
    actions = ["s3:*"]
    resources = [
      aws_s3_bucket.evidence.arn,
      "${aws_s3_bucket.evidence.arn}/*",
    ]

    principals {
      type        = "*"
      identifiers = ["*"]
    }

    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
}

resource "aws_s3_bucket_policy" "evidence" {
  bucket = aws_s3_bucket.evidence.id
  policy = data.aws_iam_policy_document.evidence_bucket.json
}

output "archive_bucket" {
  value = aws_s3_bucket.archive.id
}

output "evidence_bucket" {
  value = aws_s3_bucket.evidence.id
}

output "archive_kms_key_arn" {
  value = aws_kms_key.archive.arn
}

output "archive_key_deletion_scp" {
  value = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid      = "DenyArchiveKeyDeletion"
      Effect   = "Deny"
      Action   = ["kms:ScheduleKeyDeletion", "kms:DeleteImportedKeyMaterial"]
      Resource = aws_kms_key.archive.arn
    }]
  })
}
