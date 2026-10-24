-- create table users
CREATE TABLE "users" (
  "id" text PRIMARY KEY,
  "created_at" timestamptz NOT NULL,
  "email" text NOT NULL UNIQUE,
  "name" text NOT NULL
);
