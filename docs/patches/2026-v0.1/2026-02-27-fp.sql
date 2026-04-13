-- new statuses for issue resolution
INSERT INTO "statuses" ("statusId", "title", "alias") VALUES
    (4, 'Valid', 'valid'),
    (5, 'FalsePositive', 'falsePositive'),
    (6, 'Ignored', 'ignored');

-- migrate data from isFalsePositive to statusId
UPDATE "issues" SET "statusId" = 4 WHERE "isFalsePositive" = false;
UPDATE "issues" SET "statusId" = 5 WHERE "isFalsePositive" = true;

-- drop old column and add new index
DROP INDEX IF EXISTS "IX_issues_isFalsePositive";
ALTER TABLE "issues" DROP COLUMN "isFalsePositive";

CREATE INDEX "IX_issues_statusId" ON "issues" ("statusId");
