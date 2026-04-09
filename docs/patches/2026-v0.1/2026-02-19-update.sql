alter table "taskTrackers"
    add "url" varchar(255) NOT NULL DEFAULT '',
    alter column "authToken" drop not null;

alter table projects
    add column "instructions" text;

alter table "issues"
    add column "localId" varchar(16);


-- indexes

CREATE INDEX "IX_reviews_projectId" ON "reviews" ("projectId");
CREATE INDEX "IX_reviews_externalId" ON "reviews" ("externalId");
CREATE INDEX "IX_reviews_reviewId" ON "reviewFiles" ("reviewId");
CREATE INDEX "IX_issues_isFalsePositive" ON "issues" ("isFalsePositive");
CREATE INDEX "IX_issues_reviewId" ON "issues" ("reviewId");
CREATE INDEX "IX_issues_reviewFileId" ON "issues" ("reviewFileId");


ALTER TABLE "issues" ADD CONSTRAINT "Ref_issues_to_reviews" FOREIGN KEY ("reviewId")
    REFERENCES "reviews"("reviewId")
        MATCH SIMPLE
    ON DELETE NO ACTION
    ON UPDATE NO ACTION
    NOT DEFERRABLE;
