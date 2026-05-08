ALTER TABLE "issues" ADD COLUMN "archivedAt" timestamptz;
CREATE INDEX "IX_issues_archivedAt" ON "issues" ("archivedAt");
