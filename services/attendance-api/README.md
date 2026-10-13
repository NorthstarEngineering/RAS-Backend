# Attendance API

Independent Go module for employees, attendance, deductions, and overtime. It listens on port 9003 and exposes `GET /health`; Swagger UI is available at `/swagger/index.html`.

Attendance owns `employees`, `attendance_records`, `deductions`, and `overtime`; it reads Branch-owned `branches`. Protected API calls require Auth-issued Bearer JWTs; caller-supplied identity headers are ignored. See [the API reference](../../docs/api.md) and [security notes](../../docs/security.md).

Run through root Docker Compose so the shared Goose migration history is applied first. See [development docs](../../docs/development.md). Never run service-specific legacy migration files; they are archived for audit only.
