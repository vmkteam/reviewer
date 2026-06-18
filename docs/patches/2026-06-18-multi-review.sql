-- Multi-review (fusion): per-project panel + judge, fusion/member review roles, and
-- per-issue provenance. See docs/llm/MultiReview.md. DDL only — existing projects keep an
-- empty panel (runnerProfileIds = []) + null judge = unchanged single-review behaviour.

-- reviews: role of the row, and a member's link to its fusion parent.
ALTER TABLE "reviews" ADD COLUMN "parentReviewId" integer;
ALTER TABLE "reviews" ADD COLUMN "reviewRole" varchar(16) NOT NULL DEFAULT 'single';
ALTER TABLE "reviews" ADD CONSTRAINT "Ref_reviews_to_reviews" FOREIGN KEY ("parentReviewId")
	REFERENCES "reviews"("reviewId")
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;
CREATE INDEX "ix_reviews_parentReviewId" ON "reviews" ("parentReviewId");

-- issues: provenance for fused issues — the member model labels that flagged each issue
-- (["gpt-5.5","deepseek-v4-pro"], "judge" for verified net-new). Empty for single/member.
ALTER TABLE "issues" ADD COLUMN "sources" jsonb NOT NULL DEFAULT '[]';

-- projects: additional panel members (runnerProfileIds = []int of runnerProfileId) and the
-- judge. Full panel = runnerProfileId + runnerProfileIds; multi-review fires only when
-- judgeRunnerProfileId is set. runnerProfileIds FK integrity is enforced in code (VT layer).
ALTER TABLE "projects" ADD COLUMN "runnerProfileIds" jsonb NOT NULL DEFAULT '[]';
ALTER TABLE "projects" ADD COLUMN "judgeRunnerProfileId" integer;
ALTER TABLE "projects" ADD CONSTRAINT "Ref_projects_to_runnerProfiles_judge" FOREIGN KEY ("judgeRunnerProfileId")
	REFERENCES "runnerProfiles"("runnerProfileId")
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;
CREATE INDEX "ix_projects_judgeRunnerProfileId" ON "projects" ("judgeRunnerProfileId");
