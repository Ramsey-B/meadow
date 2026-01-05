-- Rollback criteria-based relationships

-- Remove FK constraint from staged_relationships
ALTER TABLE staged_relationships DROP CONSTRAINT IF EXISTS fk_staged_rel_criteria;

-- Drop criteria tables
DROP TABLE IF EXISTS staged_relationship_criteria_matches CASCADE;
DROP TABLE IF EXISTS staged_relationship_criteria CASCADE;
