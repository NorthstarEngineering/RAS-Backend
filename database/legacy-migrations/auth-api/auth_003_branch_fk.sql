-- Module: auth | run AFTER the branch module migration that creates `branches`.

ALTER TABLE users
    ADD CONSTRAINT users_branch_fk
    FOREIGN KEY (branch_id) REFERENCES branches(id);
