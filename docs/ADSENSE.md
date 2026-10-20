# AdSense deployment

The live verification release identifies publisher `pub-9693175657690620` through
`/ads.txt` and an ownership meta tag. Verification is independent of advertising.
The serving integration is disabled by default and needs account configuration
before it can be activated.

## Account configuration required

In the existing AdSense account:

1. Record Google's current site status. The integration can be deployed while
   review is pending; only Google can approve the site to serve ads.
2. Create a responsive Display ad unit and copy its `data-ad-slot` identifier.
   Keep Auto ads off: the app places one labeled unit on the home page and one
   on paper pages, away from search, PDF, and graph controls.
3. Publish Google's European regulations message for arxiv.gg with a first-page
   decline choice, English, and `https://arxiv.gg/privacy` as the policy URL.
   Enable Consent Mode support including Analytics.
4. Publish the US state regulations message for all supported current and future
   states. Retain Google's default Do Not Sell or Share link.

The public publisher ID is not an account credential and cannot reveal approval
status or create ad units. These steps require the signed-in AdSense dashboard.

## Server configuration

Set these together in the release configuration only after account setup:

```dotenv
GOOGLE_ADSENSE_CLIENT_ID=ca-pub-9693175657690620
GOOGLE_ADSENSE_AD_SLOT=<real numeric data-ad-slot from the dashboard>
GOOGLE_ADSENSE_ADS_ENABLED=true
GOOGLE_ADSENSE_CONSENT_READY=true
```

Both flags must be exactly `true`; a valid client and slot are also required.
Local mode never loads advertising. Empty, invalid, or disabled configuration
retains ownership verification without the ad script or empty ad placeholders.
To stop ads, set `GOOGLE_ADSENSE_ADS_ENABLED=false` and redeploy the app.

Ad scripts run only on the home page, paper pages, and the privacy page. Privacy
loads the consent controls but has no ad unit. Search, account, login, API,
admin, feedback, category, and author pages do not load the AdSense script.
The privacy notice switches to the advertising disclosure when enabled.

Google's published messages are delivered by the AdSense tag. The application
uses Google's TCF API to expose the European consent-reopening control and
leaves the US opt-out link to Google. Reopening consent disables Analytics until
a new choice is made and the page reloads. On these pages, Analytics starts only
when Google's consent API reports Analytics storage granted or not applicable;
unknown, denied, or unconfigured states do not start it. While this integration
is configured, Analytics is suppressed on all other pages, which do not load
the CMP; navigating there must not bypass a consent refusal.

## Security and caching

Ad-enabled HTML uses a fresh 256-bit nonce for every script and Google's
nonce-based strict CSP. The policy retains object, base, framing, and form
protections. Google's supported policy allows dynamic ad resources, so the
existing resource-domain restrictions are not applied to those pages. The
original CSP remains on other pages and whenever advertising is disabled.

Nonce-bearing HTML sends `Cache-Control: private, no-store` to prevent nonce
reuse through the response cache or intermediary caches. This reduces HTML
cache hit rates after activation; monitor render latency and application load.
Database/query caches are unchanged.

## Validation and release

Follow `docs/DEPLOYMENT_RUNBOOK_2026-05-15.md`, including backup validation and an
exact-image rollback. Preserve the existing PostgreSQL container and volume.
No database migrations are part of this change.

The regression tests cover disabled configuration, private-page exclusion,
matching nonces on every rendered script, nonce freshness through the response
cache, placement counts, and JavaScript consent/revocation behavior. Run:

```sh
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

After a real ad slot and published messages are available, validate on desktop
and mobile before claiming completion:

- Home and paper pages: one async AdSense loader and one labeled responsive unit;
  no overflow or overlap with navigation, search controls, or PDF links.
- Privacy page: consent controls, updated notice, and no ad unit.
- Login/account/admin/search: no AdSense requests.
- Google test parameters `?fc=alwaysshow&fctype=gdpr` and
  `?fc=alwaysshow&fctype=ccpa`: exercise consent, refusal, reopening, and US opt-out.
  These tests use real consent records; use a separate browser test profile.
- Check CSP violations, ad-blocked and unfilled cases, and Analytics consent
  behavior. Do not click ads or create repeated production impressions.
- Confirm public search and PostgreSQL health after deployment.

Automated tests use synthetic data and do not prove that Google has approved the
site, published messages, or filled an ad. Browser, mobile, and real CMP tests
remain necessary after account setup; do not report ads as live before observing
successful serving in the approved configuration.

## Google references

- [AdSense CSP integration](https://support.google.com/adsense/answer/16283098)
- [Responsive ad parameters](https://support.google.com/adsense/answer/9183460)
- [European consent messages](https://support.google.com/adsense/answer/10960768)
- [US state messages](https://support.google.com/adsense/answer/10960771)
- [Privacy and messaging JavaScript API](https://developers.google.com/funding-choices/fc-api-docs)
