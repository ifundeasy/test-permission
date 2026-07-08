-- ---------------------------------------------------------------------------
-- This is YOUR application's data. Cedar never sees this table directly.
-- The authz-service queries it, then builds Cedar entities from the rows.
-- ---------------------------------------------------------------------------
DROP TABLE IF EXISTS memberships, documents, folders, roles, groups, users CASCADE;

CREATE TABLE users   ( id text PRIMARY KEY );
CREATE TABLE groups  ( id text PRIMARY KEY );
CREATE TABLE roles   ( id text PRIMARY KEY );

CREATE TABLE folders (
  id               text PRIMARY KEY,
  parent_folder_id text REFERENCES folders(id)      -- folders can nest (ReBAC hierarchy)
);

CREATE TABLE documents (
  id           text PRIMARY KEY,
  owner_id     text REFERENCES users(id),
  confidential boolean NOT NULL DEFAULT false,       -- attribute used by an ABAC rule
  folder_id    text REFERENCES folders(id)           -- which folder the doc lives in
);

-- Generic membership: a user belongs to a Group OR a Role.
CREATE TABLE memberships (
  user_id     text REFERENCES users(id),
  parent_type text NOT NULL,                          -- 'Group' | 'Role'
  parent_id   text NOT NULL,
  PRIMARY KEY (user_id, parent_type, parent_id)
);

-- ---------------------------- seed data ------------------------------------
INSERT INTO users(id)  VALUES ('alice'),('bob'),('dave'),('carol');
INSERT INTO groups(id) VALUES ('engineering');
INSERT INTO roles(id)  VALUES ('admin');

INSERT INTO folders(id, parent_folder_id) VALUES ('eng', NULL);

INSERT INTO documents(id, owner_id, confidential, folder_id) VALUES
  ('design', 'alice', false, 'eng'),
  ('secret', 'alice', true , 'eng');

INSERT INTO memberships(user_id, parent_type, parent_id) VALUES
  ('alice', 'Group', 'engineering'),
  ('bob'  , 'Group', 'engineering'),
  ('carol', 'Role' , 'admin');
