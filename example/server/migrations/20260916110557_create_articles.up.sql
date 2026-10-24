-- create table articles
CREATE TABLE "articles" (
  "id" text PRIMARY KEY,
  "author_id" text NOT NULL,
  "body" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  "published" boolean NOT NULL,
  "slug" text NOT NULL UNIQUE,
  "title" text NOT NULL,
  "updated_at" timestamptz NOT NULL
);
