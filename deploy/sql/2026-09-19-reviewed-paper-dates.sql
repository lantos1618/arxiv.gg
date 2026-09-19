-- AXG-003: reviewed against the official arXiv submission histories on 2026-09-19.
-- https://arxiv.org/abs/1706.03762  v1 2017-06-12, v7 2023-08-02
-- https://arxiv.org/abs/1312.6114   v1 2013-12-20, v11 2022-12-10
-- https://arxiv.org/abs/1810.04805  v1 2018-10-11, v2 2019-05-24
-- Guard both observed old values, so a later metadata refresh is never overwritten.
-- Preserve fetched_at: correcting dates does not make these newly fetched papers.
SET lock_timeout = '3s';
SET statement_timeout = '10s';
UPDATE papers AS p
SET created = fix.first_submitted, updated = fix.latest_revision
FROM (VALUES
 ('1706.03762', TIMESTAMPTZ '2023-08-02 00:00:00+00', TIMESTAMPTZ '2023-08-03 00:00:00+00', TIMESTAMPTZ '2017-06-12 00:00:00+00', TIMESTAMPTZ '2023-08-02 00:00:00+00'),
 ('1312.6114', TIMESTAMPTZ '2022-12-10 00:00:00+00', TIMESTAMPTZ '2022-12-13 00:00:00+00', TIMESTAMPTZ '2013-12-20 00:00:00+00', TIMESTAMPTZ '2022-12-10 00:00:00+00'),
 ('1810.04805', TIMESTAMPTZ '2019-05-24 00:00:00+00', TIMESTAMPTZ '2019-05-28 00:00:00+00', TIMESTAMPTZ '2018-10-11 00:00:00+00', TIMESTAMPTZ '2019-05-24 00:00:00+00')
) AS fix(id, old_created, old_updated, first_submitted, latest_revision)
WHERE p.id = fix.id AND p.created = fix.old_created AND p.updated = fix.old_updated;
RESET lock_timeout;
RESET statement_timeout;
