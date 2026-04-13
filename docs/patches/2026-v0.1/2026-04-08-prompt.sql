-- v0.2: suggested fixes + effort estimate + anti-slop
ALTER TABLE "issues" ADD COLUMN "suggestedFix" text;
ALTER TABLE "reviews" ADD COLUMN "effortMinutes" integer;
ALTER TABLE "reviews" ADD COLUMN "aiSlopScore" real;
