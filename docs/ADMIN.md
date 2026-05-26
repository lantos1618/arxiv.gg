# arXiv.gg Admin

The admin area is intentionally small and read-only.

## Routes

- `/admin` - ops overview, embedding coverage, job queue, plan model, and explicit placeholders.
- `/admin/users` - real Google users from the `users` table and simple plan counts.
- `/admin/audit` - admin page-view audit log from `admin_audit_log`.

## Access

Admin access supports both paths:

- Google sign-in when the email is listed in `ADMIN_EMAILS`.
- Existing `ADMIN_TOKEN` for API/admin-token fallback.

`ADMIN_EMAILS` accepts comma, space, semicolon, or newline separated emails.

## Placeholders

Rows marked `PLACEHOLDER` are deliberately not fake data. They show areas that are not wired into the app yet:

- Cloudflare, Google Search Console, Bing, and GA traffic.
- Payments/Stripe/billing events.
- Remote GPU worker health.
- Per-user saved searches and feature usage.

Those should only become normal rows after the app has real data sources for them.
