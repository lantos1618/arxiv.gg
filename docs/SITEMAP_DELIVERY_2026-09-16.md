# Sitemap delivery and webmaster follow-up — 2026-09-16

The catalog exposes its canonical paper URLs through `/sitemap.xml`, which references `/sitemap-static.xml` and numbered files such as `/sitemaps/papers-1.xml`. At the reviewed production scale, the index contained 63 paper shards. The XML was valid, but representative first/middle/last requests took roughly 1.9/11.1/26.7 seconds. A full shard contained 50,000 URLs and approximately 8.6 MB of uncompressed XML, exceeding the generic response cache's 2 MiB item limit.

## Delivery changes

- Find the requested page boundary using the primary-key index, then load only the selected page's `id` and `updated` fields. Both steps use one SQL statement/snapshot. This removes the observed wide-table scan and sort at deep offsets; the boundary lookup still walks preceding index entries.
- Keep public sitemap XML in a dedicated cache: one-hour freshness, 64 MiB retained bodies, 12 MiB maximum cached document, and at most 128 entries. Least-recently-used/expired entries are evicted; valid oversized documents are served completely but not retained.
- Coalesce concurrent requests for the same document. Limit all cold sitemap builds together to two, with a 20-second timeout covering queueing and database generation. Failed builds are not cached. Timed-out builds return a retryable 503.
- Share the complete representation between HEAD and GET. Return ETag and Last-Modified validators on first and later responses, using the standard HTTP conditional-request implementation. HEAD suppresses the body without losing its cached representation.
- Keep sitemap bodies outside the ordinary HTML response cache. XML is public and does not depend on cookies, account identity, or request query parameters. Other routes retain their authentication/cache isolation.
- Normalize numeric shard aliases into one cache entry and reject offsets that would overflow before accessing the database.

The catalog schema, canonical URLs, 50,000-URL shard limit, and XML content conventions remain compatible. This work improves delivery; it does not establish the cause of Google's historical discovery state or guarantee indexing of already-crawled papers.

## Search Console after deployment

1. In the arxiv.gg Domain property, live-inspect `https://arxiv.gg/sitemap.xml` and `https://arxiv.gg/sitemaps/papers-1.xml`. Confirm the actual Google fetch succeeds and crawling is allowed. A local browser extension blocking navigation is not Google's fetch result.
2. Submit the verified root sitemap once through Sitemaps. Do not delete the property, mass-submit all paper URLs, or repeatedly resubmit an unchanged sitemap.
3. Record the new last-read time, child-sitemap list, discovered URL counts, and any precise processing error. These provide evidence that discovery is working. Crawl/indexing progress is a separate check.
4. Track a small representative cohort of paper pages in URL Inspection: current rendered page, successful fetch, indexing status/reason, and Google-selected canonical. Do not assume the original arxiv.org URL was selected without that evidence.

Official workflow: https://support.google.com/webmasters/answer/7451001?hl=en
Indexing limitations: https://developers.google.com/search/docs/fundamentals/how-search-works

## Google Analytics validation

Keep the deployed collection fixes and current filters unchanged while testing. Use Tag Assistant on the testing device and Admin → Data display → DebugView to identify a small number of public-page visits with a unique, non-sensitive query marker in `page_location`. Test signed-out and signed-in readers, including actual US/EU network connections where available. Confirm one intended `page_view` per load, then engagement after genuine visible-page use and navigation. Login/account/admin pages intentionally omit the tag.

Inspect collection requests and actual property reporting separately. HTTP 204 alone does not establish final reporting identity. Do not enable debug mode for all visitors, activate a country exclusion, or convert Cloudflare request counts into human readership estimates.

Official DebugView workflow: https://support.google.com/analytics/answer/7201382

## Validation

Focused tests cover pagination including legacy IDs and empty/canceled reads, large complete sitemap responses, real-handler HEAD-first reuse, HTTP revalidation, failure/cancellation behavior, cache bounds, and existing authenticated-page cache isolation. Release checks also compare first/middle/last live sitemap delivery and confirm that the database schema is unchanged.
