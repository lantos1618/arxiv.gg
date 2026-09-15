# Analytics collection follow-up — 2026-09-15

The site omitted Google Analytics from every signed-in page, including ordinary public paper pages. Public-page measurement now uses the same configured tag for signed-in and anonymous readers. Login, account, OAuth, API setup, and admin pages remain excluded. No account ID or profile fields are added to the tag configuration.

This corrects missing signed-in readership. It does not explain missing signed-out visits; those have a separate reproduced collection failure described below. The country card alone does not identify a collection failure or establish zero European visitors: it shows top countries, and processed reports can lag collection. Browser blocking, report filters, and VPN egress remain relevant to checking the user's particular device.

Regression checks render public and sensitive pages for both signed-in and anonymous users. Existing JavaScript checks verify immediate asynchronous initialization and one loader/configuration per page.

Browser checks and deployment results are recorded under `/home/ubuntu/arxiv-ops/analytics-readers-20260915/`.

## Reproduced anonymous collection failure

Controlled Chromium checks using anonymous desktop and Pixel 7 contexts loaded the live page and its configured Google tag. The current tag attempted a `page_view` to `https://analytics.google.com/g/collect`; the live `connect-src` permitted only `www.google-analytics.com` and `region1.google-analytics.com`, so the browser blocked that request.

An isolated response-header comparison explicitly allowed `https://analytics.google.com`, together with Google's documented analytics subdomain families and tag-manager connection host. The primary page-view request then reached the browser request interceptor without a CSP violation. Every measurement request was aborted before transmission, so these checks did not send test visits to the production property. A wildcard such as `https://*.analytics.google.com` alone did not match the parent `analytics.google.com` host.

The production correction adds those core measurement destinations. Optional advertising destinations remain outside this change. This confirms a site-side cause of dropped anonymous events; it does not establish that all missing visits or the Singapore concentration have that cause. The test browser did not use EU VPN egress and cannot confirm the user's actual phone settings or GA property reports.

Source: [Google's analytics CSP requirements](https://developers.google.com/tag-platform/security/guides/csp#google_analytics).

Validation passed: `go test ./... -count=1`, `go test -race ./... -count=1`, `go vet ./...`, and `git diff --check`. The controlled browser comparison reproduced the anonymous failure before the policy correction and allowed the primary request afterward.
