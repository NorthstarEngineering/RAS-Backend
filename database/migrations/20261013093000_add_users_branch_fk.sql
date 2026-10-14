-- +goose Up
ALTER TABLE users
    ADD CONSTRAINT users_branch_fk FOREIGN KEY (branch_id) REFERENCES branches(id);

-- +goose Down
ALTER TABLE users DROP CONSTRAINT users_branch_fk;
