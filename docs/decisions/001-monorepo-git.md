# RAS-backend Git history consolidation record

Date: 2026-10-13
Workspace: `C:\Users\myfin\OneDrive\Desktop\RAS-backend`

Before restructuring, each application repository was clean on `main`; each had only the local `main` plus `origin/main` and `origin/HEAD` refs. No root repository existed. No remote was modified and no commit was created.

| Application | Original HEAD | Branch | Origin (fetch/push) | Verified local bundle |
|---|---|---|---|---|
| auth-api | `f7b7a9bd1b2e7deb3771d948032e218b9c2bf3c9` | `main` | `https://github.com/malakhafez/auth-api.git` | `.git-preservation/auth-api.bundle` |
| branch-api | `fa3b4ab5031e39cf91e8d99117f3d95ecd8462c6` | `main` | `https://github.com/malakhafez/branch-api` | `.git-preservation/branch-api.bundle` |
| attendance-api | `63a0cc2ab37dd024267a9b396163266f7bf5e6ed` | `main` | `https://github.com/malakhafez/attendance-api` | `.git-preservation/attendance-api.bundle` |

Each bundle was created with `git bundle create <path> --all` and verified with `git bundle verify`; each contains the original branch and remote-tracking refs and complete reachable history. The original `.git` directories are retained under `.git-preservation/` as local ignored data. These bundles and metadata are not intended to be committed or pushed because historical revisions can contain obsolete configuration and credentials.

The consolidated repository is based on the existing `NorthstarEngineering/RAS-Backend` GitHub `main` branch. The first set of changes is arranged as stacked feature branches so each pull request can be reviewed and merged in order. To recover a source repository's history, use `git clone .git-preservation/<app>.bundle <destination>` or fetch the bundle refs into a separate repository. The local preservation bundles remain excluded from commits.
