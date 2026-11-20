CREATE TABLE IF NOT EXISTS posts (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    author_id bigint NOT NULL,
    body text NOT NULL,
    comments_closed boolean NOT NULL DEFAULT false
);

CREATE TABLE IF NOT EXISTS comments (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    post_id bigint NOT NULL REFERENCES posts(id),
    parent_id bigint,
    body varchar(2000) NOT NULL,
    UNIQUE (post_id, id),
    FOREIGN KEY (post_id, parent_id) REFERENCES comments(post_id, id)
);

CREATE INDEX IF NOT EXISTS comments_branch_page ON comments(post_id, parent_id, id);

