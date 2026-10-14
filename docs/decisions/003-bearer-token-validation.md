# Decision 003: services validate signed Bearer tokens locally

**Status:** Accepted
**Date:** 2026-10-13

Branch and Attendance validate Auth's HS256 signed JWT locally using the shared secret, issuer, expiry, subject, role, branch claim, and must-change-password state. They do not trust caller-supplied identity headers and do not call Auth on every request. Branch writes require CEO; Branch Manager reads and Attendance operations are restricted by role and branch.

This changes clients that depended on unsigned `X-User-ID`, `X-User-Role`, or `X-Branch-ID` headers: they must provide an Auth-issued Bearer token. HS256 means every verifier can sign tokens; a future asymmetric signing change requires a separately coordinated token rollout. Stateless access tokens remain valid until expiry after disablement/revocation.
