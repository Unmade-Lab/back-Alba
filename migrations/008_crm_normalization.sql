-- =============================================
-- CRM NORMALIZATION: IDs INSTEAD OF STRINGS
-- =============================================

-- 1. Users: Link to Department
ALTER TABLE users ADD COLUMN IF NOT EXISTS department_id UUID REFERENCES departments(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_users_department_id ON users(department_id);

-- 2. Deals: Link to Pipeline & Stage
-- Keep 'stage' (string) for now to prevent breaking existing code, but add IDs
ALTER TABLE deals ADD COLUMN IF NOT EXISTS pipeline_id UUID REFERENCES pipelines(id) ON DELETE SET NULL;
ALTER TABLE deals ADD COLUMN IF NOT EXISTS stage_id UUID REFERENCES stages(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_deals_pipeline_id ON deals(pipeline_id);
CREATE INDEX IF NOT EXISTS idx_deals_stage_id ON deals(stage_id);

-- 3. Update existing deals to attempt to map 'stage' string to a real 'stage_id' 
-- (Best effort: match by name within the same workspace)
DO $$
BEGIN
    UPDATE deals d
    SET stage_id = s.id, pipeline_id = s.pipeline_id
    FROM stages s
    INNER JOIN pipelines p ON s.pipeline_id = p.id
    WHERE LOWER(d.stage) = LOWER(s.name) 
      AND d.workspace_id = p.workspace_id
      AND d.stage_id IS NULL;
END $$;
