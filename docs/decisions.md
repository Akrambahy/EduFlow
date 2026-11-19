# Decisions (SRS section 15)

| ID | Decision | Status |
|---|---|---|
| DEC-01 | Admin-managed accounts only; there is no sign-up endpoint. | Confirmed |
| DEC-02 | Only admins assign roles, and nobody can change their own roles or deactivate themselves. | Confirmed |
| DEC-03 | One track per course (`courses.track_id`). | Confirmed |
| DEC-04 | Versioned grade policy per course. Defaults: quizzes 20, assignments 40, attendance 20, projects 20. A component with no data is excluded and its weight is redistributed. Missing work counts as 0 once the deadline closes; pending work is excluded. | Confirmed |
| DEC-05 | A check-in counts as late after `startsAt` + 10 minutes. The grace period is configurable per window, and the server's clock decides. | Confirmed |
| DEC-06 | QR tokens rotate every 30 s; the previous step is still accepted to allow for scan latency. | Confirmed |
| DEC-07 | Attendance corrections are made by the course instructor or an admin, require a reason, and are audited with the before and after status. Closing the window marks absent students. | Confirmed |
| DEC-08 | Files up to 20 MB (`MAX_UPLOAD_MB`). Allowed types: pdf, docx, pptx, xlsx, zip, txt, md, png, jpg. Each file's content is checked against its extension. | Confirmed |
| DEC-09 | Storage interface with a local adapter; an S3-compatible adapter will be added at deployment. | Confirmed |
| DEC-10 | Email provider: none in the MVP, so password recovery is deferred. | Deferred |
| DEC-11 | GitHub: repository linking plus optional metadata. OAuth is deferred. GitHub never affects grades. | Confirmed |
| DEC-12 | The public certificate page shows the code, status, student name, course and issue date only. | Confirmed |
| DEC-13 | Deployment target: not chosen yet. A Docker image is built in CI. | Open |
| DEC-14 | Performance targets: not set yet. | Open |
| DEC-15 | Blockchain is out of the MVP. | Confirmed |

## Other agreed choices

**API**
- Prefix: `/api/v1` only. The frontend sets `VITE_API_BASE_URL=/api/v1`.
- Response shape: no envelope. Errors use `{message, code, details?}`. Lists return arrays, with the total in `X-Total-Count`. Pages are capped at 100 items, with a default of 20.

**Authentication**
- The refresh token is an httpOnly cookie that rotates on each use; reusing an old one is detected. Access tokens last 2 hours until the frontend adds refresh-on-401.

**Learning**
- Lesson locking uses a `sequential_unlock` flag on each course. Whether a lesson is locked is computed in SQL, never stored.
- Late assignments follow a policy set per assignment: reject, accept, or penalty with a cutoff.
- Resubmission is allowed until the work is graded, unless the grader returns it for revision.
- Quiz answers are revealed according to a setting on each quiz: after submission, after the quiz closes (the default), or never.

**Standing and certificates**
- Probation: grade below 50% or attendance below 60%.
- Warning: grade below 60%, attendance below 75%, or 2 or more missing items.
- Certificate eligibility (configurable per course): all lessons complete, grade of at least 60, attendance of at least 75, and not on probation. Only admins issue certificates.
