-- Migration 010: Add enrichment fields to companies and wire notifications
-- Adds description field to companies for AI enrichment feature.

ALTER TABLE companies ADD COLUMN IF NOT EXISTS description TEXT;
