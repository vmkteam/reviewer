ALTER TABLE "prompts" DROP CONSTRAINT "Ref_prompts_to_statuses";

ALTER TABLE "taskTrackers" DROP CONSTRAINT "Ref_taskTrackers_to_statuses";

ALTER TABLE "slackChannels" DROP CONSTRAINT "Ref_slackChannels_to_statuses";

ALTER TABLE "projects" DROP CONSTRAINT "Ref_projects_to_prompts";

ALTER TABLE "projects" DROP CONSTRAINT "Ref_projects_to_taskTrackers";

ALTER TABLE "projects" DROP CONSTRAINT "Ref_projects_to_slackChannels";

ALTER TABLE "projects" DROP CONSTRAINT "Ref_projects_to_statuses";

ALTER TABLE "reviews" DROP CONSTRAINT "Ref_reviews_to_projects";

ALTER TABLE "reviews" DROP CONSTRAINT "Ref_reviews_to_statuses";

ALTER TABLE "reviews" DROP CONSTRAINT "Ref_reviews_to_prompts";

ALTER TABLE "reviewFiles" DROP CONSTRAINT "Ref_reviewFiles_to_reviews";

ALTER TABLE "reviewFiles" DROP CONSTRAINT "Ref_reviewFiles_to_statuses";

ALTER TABLE "issues" DROP CONSTRAINT "Ref_issues_to_reviewFiles";

ALTER TABLE "issues" DROP CONSTRAINT "Ref_issues_to_statuses";

ALTER TABLE "issues" DROP CONSTRAINT "Ref_issues_to_users";

ALTER TABLE "issues" DROP CONSTRAINT "Ref_issues_to_reviews";

ALTER TABLE "users" DROP CONSTRAINT "Ref_users_to_statuses";

CREATE INDEX "ix_issues_userId" ON "issues" (
	"userId"
);

CREATE INDEX "ix_projects_statusId" ON "projects" (
	"statusId"
);

CREATE INDEX "ix_prompts_statusId" ON "prompts" (
	"statusId"
);

CREATE INDEX "ix_reviewFiles_statusId" ON "reviewFiles" (
	"statusId"
);

CREATE INDEX "ix_reviews_promptId" ON "reviews" (
	"promptId"
);

CREATE INDEX "ix_slackChannels_statusId" ON "slackChannels" (
	"statusId"
);

CREATE INDEX "ix_taskTrackers_statusId" ON "taskTrackers" (
	"statusId"
);

CREATE INDEX "ix_users_statusId" ON "users" (
	"statusId"
);

CREATE INDEX "ix_projects_slackChannelId" ON "projects" (
	"slackChannelId"
);

CREATE INDEX "ix_reviews_statusId" ON "reviews" (
	"statusId"
);

CREATE INDEX "ix_projects_taskTrackerId" ON "projects" (
	"taskTrackerId"
);

CREATE INDEX "ix_projects_promptId" ON "projects" (
	"promptId"
);

ALTER TABLE "prompts" ADD CONSTRAINT "Ref_prompts_to_statuses" FOREIGN KEY ("statusId")
	REFERENCES "statuses"("statusId")
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;

ALTER TABLE "taskTrackers" ADD CONSTRAINT "Ref_taskTrackers_to_statuses" FOREIGN KEY ("statusId")
	REFERENCES "statuses"("statusId")
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;

ALTER TABLE "slackChannels" ADD CONSTRAINT "Ref_slackChannels_to_statuses" FOREIGN KEY ("statusId")
	REFERENCES "statuses"("statusId")
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;

ALTER TABLE "projects" ADD CONSTRAINT "Ref_projects_to_prompts" FOREIGN KEY ("promptId")
	REFERENCES "prompts"("promptId")
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;

ALTER TABLE "projects" ADD CONSTRAINT "Ref_projects_to_taskTrackers" FOREIGN KEY ("taskTrackerId")
	REFERENCES "taskTrackers"("taskTrackerId")
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;

ALTER TABLE "projects" ADD CONSTRAINT "Ref_projects_to_slackChannels" FOREIGN KEY ("slackChannelId")
	REFERENCES "slackChannels"("slackChannelId")
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;

ALTER TABLE "projects" ADD CONSTRAINT "Ref_projects_to_statuses" FOREIGN KEY ("statusId")
	REFERENCES "statuses"("statusId")
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;

ALTER TABLE "reviews" ADD CONSTRAINT "Ref_reviews_to_projects" FOREIGN KEY ("projectId")
	REFERENCES "projects"("projectId")
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;

ALTER TABLE "reviews" ADD CONSTRAINT "Ref_reviews_to_statuses" FOREIGN KEY ("statusId")
	REFERENCES "statuses"("statusId")
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;

ALTER TABLE "reviews" ADD CONSTRAINT "Ref_reviews_to_prompts" FOREIGN KEY ("promptId")
	REFERENCES "prompts"("promptId")
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;

ALTER TABLE "reviewFiles" ADD CONSTRAINT "Ref_reviewFiles_to_reviews" FOREIGN KEY ("reviewId")
	REFERENCES "reviews"("reviewId")
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;

ALTER TABLE "reviewFiles" ADD CONSTRAINT "Ref_reviewFiles_to_statuses" FOREIGN KEY ("statusId")
	REFERENCES "statuses"("statusId")
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;

ALTER TABLE "issues" ADD CONSTRAINT "Ref_issues_to_reviewFiles" FOREIGN KEY ("reviewFileId")
	REFERENCES "reviewFiles"("reviewFileId")
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;

ALTER TABLE "issues" ADD CONSTRAINT "Ref_issues_to_statuses" FOREIGN KEY ("statusId")
	REFERENCES "statuses"("statusId")
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;

ALTER TABLE "issues" ADD CONSTRAINT "Ref_issues_to_users" FOREIGN KEY ("userId")
	REFERENCES "users"("userId")
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;

ALTER TABLE "issues" ADD CONSTRAINT "Ref_issues_to_reviews" FOREIGN KEY ("reviewId")
	REFERENCES "reviews"("reviewId")
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;

ALTER TABLE "users" ADD CONSTRAINT "Ref_users_to_statuses" FOREIGN KEY ("statusId")
	REFERENCES "statuses"("statusId")
	ON DELETE RESTRICT
	ON UPDATE RESTRICT
	NOT DEFERRABLE;
